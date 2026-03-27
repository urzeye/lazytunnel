package app

import (
	"errors"
	"fmt"
	"net"

	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
)

type RuntimeController interface {
	Start(spec ltruntime.ProcessSpec) error
	Stop(name string) error
	Snapshot(name string) (domain.RuntimeState, bool)
	ListStates() []domain.RuntimeState
	Subscribe(buffer int) (int, <-chan ltruntime.Event)
	Unsubscribe(id int)
}

type PortChecker interface {
	CheckLocalPort(port int) error
}

type ServiceOption func(*Service)

func WithPortChecker(portChecker PortChecker) ServiceOption {
	return func(service *Service) {
		service.portChecker = portChecker
	}
}

type Service struct {
	config      domain.Config
	supervisor  RuntimeController
	portChecker PortChecker
	profiles    map[string]domain.Profile
	profileList []domain.Profile
	stacks      map[string]domain.Stack
	stackList   []domain.Stack
}

type ProfileView struct {
	Profile domain.Profile
	State   domain.RuntimeState
}

type StackStatus string

const (
	StackStatusStopped StackStatus = "stopped"
	StackStatusPartial StackStatus = "partial"
	StackStatusRunning StackStatus = "running"
)

type StackView struct {
	Stack       domain.Stack
	Members     []ProfileView
	ActiveCount int
	Status      StackStatus
}

func NewService(config domain.Config, supervisor RuntimeController, opts ...ServiceOption) (*Service, error) {
	profiles := make(map[string]domain.Profile, len(config.Profiles))
	for _, profile := range config.Profiles {
		profiles[profile.Name] = profile
	}

	stacks := make(map[string]domain.Stack, len(config.Stacks))
	for _, stack := range config.Stacks {
		stacks[stack.Name] = stack
	}

	service := &Service{
		config:      config,
		supervisor:  supervisor,
		portChecker: localhostPortChecker{},
		profiles:    profiles,
		profileList: append([]domain.Profile(nil), config.Profiles...),
		stacks:      stacks,
		stackList:   append([]domain.Stack(nil), config.Stacks...),
	}

	for _, opt := range opts {
		opt(service)
	}

	return service, nil
}

func (s *Service) Profiles() []domain.Profile {
	return append([]domain.Profile(nil), s.profileList...)
}

func (s *Service) Stacks() []domain.Stack {
	return append([]domain.Stack(nil), s.stackList...)
}

func (s *Service) ProfileViews() []ProfileView {
	states := make(map[string]domain.RuntimeState)
	for _, state := range s.supervisor.ListStates() {
		states[state.ProfileName] = state
	}

	views := make([]ProfileView, 0, len(s.profileList))
	for _, profile := range s.profileList {
		state, exists := states[profile.Name]
		if !exists {
			state = defaultRuntimeState(profile.Name)
		}

		views = append(views, ProfileView{
			Profile: profile,
			State:   state,
		})
	}

	return views
}

func (s *Service) StackViews() []StackView {
	profileViews := s.ProfileViews()
	profileViewsByName := make(map[string]ProfileView, len(profileViews))
	for _, view := range profileViews {
		profileViewsByName[view.Profile.Name] = view
	}

	views := make([]StackView, 0, len(s.stackList))
	for _, stack := range s.stackList {
		members := make([]ProfileView, 0, len(stack.Profiles))
		activeCount := 0
		for _, profileName := range stack.Profiles {
			view, exists := profileViewsByName[profileName]
			if !exists {
				continue
			}

			if isActiveStatus(view.State.Status) {
				activeCount++
			}

			members = append(members, view)
		}

		views = append(views, StackView{
			Stack:       stack,
			Members:     members,
			ActiveCount: activeCount,
			Status:      stackStatus(activeCount, len(members)),
		})
	}

	return views
}

func (s *Service) StartProfile(name string) error {
	profile, err := s.profile(name)
	if err != nil {
		return err
	}

	return s.startProfile(profile)
}

func (s *Service) StopProfile(name string) error {
	if _, err := s.profile(name); err != nil {
		return err
	}

	return s.supervisor.Stop(name)
}

func (s *Service) StartStack(name string) error {
	stack, err := s.stack(name)
	if err != nil {
		return err
	}

	pending, err := s.preflightStackStart(stack)
	if err != nil {
		return err
	}

	var errs []error
	for _, profile := range pending {
		if err := s.startPreparedProfile(profile); err != nil {
			errs = append(errs, fmt.Errorf("profile %q: %w", profile.Name, err))
		}
	}

	return errors.Join(errs...)
}

func (s *Service) StopStack(name string) error {
	stack, err := s.stack(name)
	if err != nil {
		return err
	}

	var errs []error
	for _, profileName := range stack.Profiles {
		state, exists := s.supervisor.Snapshot(profileName)
		if !exists || !isActiveStatus(state.Status) {
			continue
		}

		if err := s.supervisor.Stop(profileName); err != nil {
			errs = append(errs, fmt.Errorf("profile %q: %w", profileName, err))
		}
	}

	return errors.Join(errs...)
}

func (s *Service) ToggleProfile(name string) error {
	if state, exists := s.supervisor.Snapshot(name); exists && isActiveStatus(state.Status) {
		return s.supervisor.Stop(name)
	}

	return s.StartProfile(name)
}

func (s *Service) ToggleStack(name string) error {
	stackView, err := s.stackView(name)
	if err != nil {
		return err
	}

	if stackView.Status == StackStatusRunning {
		return s.StopStack(name)
	}

	return s.StartStack(name)
}

func (s *Service) Subscribe(buffer int) (int, <-chan ltruntime.Event) {
	return s.supervisor.Subscribe(buffer)
}

func (s *Service) Unsubscribe(id int) {
	s.supervisor.Unsubscribe(id)
}

func (s *Service) profile(name string) (domain.Profile, error) {
	profile, exists := s.profiles[name]
	if !exists {
		return domain.Profile{}, fmt.Errorf("profile %q not found", name)
	}

	return profile, nil
}

func (s *Service) stack(name string) (domain.Stack, error) {
	stack, exists := s.stacks[name]
	if !exists {
		return domain.Stack{}, fmt.Errorf("stack %q not found", name)
	}

	return stack, nil
}

func (s *Service) stackView(name string) (StackView, error) {
	for _, view := range s.StackViews() {
		if view.Stack.Name == name {
			return view, nil
		}
	}

	return StackView{}, fmt.Errorf("stack %q not found", name)
}

func (s *Service) startProfile(profile domain.Profile) error {
	if state, exists := s.supervisor.Snapshot(profile.Name); exists && isActiveStatus(state.Status) {
		return fmt.Errorf("profile %q is already active", profile.Name)
	}

	if owner, exists := s.activePortOwner(profile.Name, profile.LocalPort); exists {
		return fmt.Errorf("local port %d is already used by active profile %q", profile.LocalPort, owner)
	}

	if err := s.portChecker.CheckLocalPort(profile.LocalPort); err != nil {
		return err
	}

	return s.startPreparedProfile(profile)
}

func (s *Service) startPreparedProfile(profile domain.Profile) error {
	spec, err := BuildProcessSpec(profile)
	if err != nil {
		return err
	}

	return s.supervisor.Start(spec)
}

func (s *Service) preflightStackStart(stack domain.Stack) ([]domain.Profile, error) {
	activeStates := s.activeStatesByName()
	reservedPorts := make(map[int]string, len(activeStates))
	for profileName, state := range activeStates {
		if !isActiveStatus(state.Status) {
			continue
		}

		profile, exists := s.profiles[profileName]
		if !exists {
			continue
		}

		reservedPorts[profile.LocalPort] = profileName
	}

	pending := make([]domain.Profile, 0, len(stack.Profiles))
	var errs []error
	for _, profileName := range stack.Profiles {
		profile, err := s.profile(profileName)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if state, exists := activeStates[profile.Name]; exists && isActiveStatus(state.Status) {
			continue
		}

		if owner, exists := reservedPorts[profile.LocalPort]; exists && owner != profile.Name {
			errs = append(errs, fmt.Errorf("profile %q: local port %d is already reserved by profile %q", profile.Name, profile.LocalPort, owner))
			continue
		}

		if err := s.portChecker.CheckLocalPort(profile.LocalPort); err != nil {
			errs = append(errs, fmt.Errorf("profile %q: %w", profile.Name, err))
			continue
		}

		reservedPorts[profile.LocalPort] = profile.Name
		pending = append(pending, profile)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return pending, nil
}

func (s *Service) activePortOwner(excludedName string, port int) (string, bool) {
	for profileName, state := range s.activeStatesByName() {
		if profileName == excludedName || !isActiveStatus(state.Status) {
			continue
		}

		profile, exists := s.profiles[profileName]
		if !exists {
			continue
		}

		if profile.LocalPort == port {
			return profileName, true
		}
	}

	return "", false
}

func (s *Service) activeStatesByName() map[string]domain.RuntimeState {
	states := make(map[string]domain.RuntimeState)
	for _, state := range s.supervisor.ListStates() {
		states[state.ProfileName] = state
	}

	return states
}

func defaultRuntimeState(profileName string) domain.RuntimeState {
	return domain.RuntimeState{
		ProfileName: profileName,
		Status:      domain.TunnelStatusStopped,
	}
}

func stackStatus(activeCount, total int) StackStatus {
	switch {
	case total == 0 || activeCount == 0:
		return StackStatusStopped
	case activeCount == total:
		return StackStatusRunning
	default:
		return StackStatusPartial
	}
}

func isActiveStatus(status domain.TunnelStatus) bool {
	switch status {
	case domain.TunnelStatusStarting, domain.TunnelStatusRunning, domain.TunnelStatusRestarting:
		return true
	default:
		return false
	}
}

type localhostPortChecker struct{}

func (localhostPortChecker) CheckLocalPort(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("local port %d is unavailable: %w", port, err)
	}

	_ = listener.Close()
	return nil
}
