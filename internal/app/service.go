package app

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"

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

type CommandChecker interface {
	CheckCommand(command string) error
}

type ConfigPersister func(domain.Config) error

type ServiceOption func(*Service)

func WithPortChecker(portChecker PortChecker) ServiceOption {
	return func(service *Service) {
		service.portChecker = portChecker
	}
}

func WithCommandChecker(commandChecker CommandChecker) ServiceOption {
	return func(service *Service) {
		service.commandChecker = commandChecker
	}
}

func WithSystemCommandChecks() ServiceOption {
	return func(service *Service) {
		service.commandChecker = systemCommandChecker{}
	}
}

type Service struct {
	config         domain.Config
	supervisor     RuntimeController
	portChecker    PortChecker
	commandChecker CommandChecker
	profiles       map[string]domain.Profile
	profileList    []domain.Profile
	stacks         map[string]domain.Stack
	stackList      []domain.Stack
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

type StartReadiness string

const (
	StartReadinessReady   StartReadiness = "ready"
	StartReadinessWarning StartReadiness = "warning"
	StartReadinessActive  StartReadiness = "active"
	StartReadinessBlocked StartReadiness = "blocked"
)

type ProfileStartAnalysis struct {
	Name     string
	Status   StartReadiness
	Problems []string
	Warnings []string
}

type StackMemberStartAnalysis struct {
	ProfileName string
	Status      StartReadiness
	Problems    []string
	Warnings    []string
}

type StackStartAnalysis struct {
	Name         string
	Members      []StackMemberStartAnalysis
	ReadyCount   int
	WarningCount int
	ActiveCount  int
	BlockedCount int
}

type RemoveProfileResult struct {
	Name              string
	WasActive         bool
	ReferencingStacks []string
	UpdatedStacks     int
	RemovedStacks     int
}

type RemoveStackResult struct {
	Name string
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
		config:         config,
		supervisor:     supervisor,
		portChecker:    localhostPortChecker{},
		commandChecker: noopCommandChecker{},
		profiles:       profiles,
		profileList:    append([]domain.Profile(nil), config.Profiles...),
		stacks:         stacks,
		stackList:      append([]domain.Stack(nil), config.Stacks...),
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

func (s *Service) RestartProfile(name string) error {
	profile, err := s.profile(name)
	if err != nil {
		return err
	}

	if _, exists := s.supervisor.Snapshot(name); exists {
		if err := s.supervisor.Stop(name); err != nil {
			return fmt.Errorf("stop profile %q before restart: %w", name, err)
		}
	}

	return s.startProfile(profile)
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

func (s *Service) RestartStack(name string) error {
	stack, err := s.stack(name)
	if err != nil {
		return err
	}

	var errs []error
	for _, profileName := range stack.Profiles {
		if _, exists := s.supervisor.Snapshot(profileName); !exists {
			continue
		}
		if err := s.supervisor.Stop(profileName); err != nil {
			errs = append(errs, fmt.Errorf("profile %q: %w", profileName, err))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return s.StartStack(name)
}

func (s *Service) AnalyzeProfileStart(name string) (ProfileStartAnalysis, error) {
	profile, err := s.profile(name)
	if err != nil {
		return ProfileStartAnalysis{}, err
	}

	analysis := ProfileStartAnalysis{Name: profile.Name}
	analysis.Warnings = append(analysis.Warnings, profileStartWarnings(profile)...)
	if state, exists := s.supervisor.Snapshot(profile.Name); exists && isActiveStatus(state.Status) {
		analysis.Status = StartReadinessActive
		return analysis, nil
	}

	if err := s.commandChecker.CheckCommand(commandForProfile(profile)); err != nil {
		analysis.Problems = append(analysis.Problems, err.Error())
	}

	if err := validateProfileStart(profile); err != nil {
		analysis.Problems = append(analysis.Problems, err.Error())
	}

	if localPort, managed := managedLocalPort(profile); managed {
		if owner, exists := s.activePortOwner(profile.Name, localPort); exists {
			analysis.Problems = append(
				analysis.Problems,
				fmt.Sprintf("local port %d is already used by active profile %q", localPort, owner),
			)
		} else if err := s.portChecker.CheckLocalPort(localPort); err != nil {
			analysis.Problems = append(analysis.Problems, err.Error())
		}
	}

	if len(analysis.Problems) > 0 {
		analysis.Status = StartReadinessBlocked
		return analysis, nil
	}

	if len(analysis.Warnings) > 0 {
		analysis.Status = StartReadinessWarning
		return analysis, nil
	}

	analysis.Status = StartReadinessReady
	return analysis, nil
}

func (s *Service) AnalyzeStackStart(name string) (StackStartAnalysis, error) {
	stack, err := s.stack(name)
	if err != nil {
		return StackStartAnalysis{}, err
	}

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

		if localPort, managed := managedLocalPort(profile); managed {
			reservedPorts[localPort] = profileName
		}
	}

	analysis := StackStartAnalysis{
		Name:    stack.Name,
		Members: make([]StackMemberStartAnalysis, 0, len(stack.Profiles)),
	}

	for _, profileName := range stack.Profiles {
		member := StackMemberStartAnalysis{ProfileName: profileName}

		profile, err := s.profile(profileName)
		if err != nil {
			member.Status = StartReadinessBlocked
			member.Problems = append(member.Problems, err.Error())
			analysis.BlockedCount++
			analysis.Members = append(analysis.Members, member)
			continue
		}

		if state, exists := activeStates[profile.Name]; exists && isActiveStatus(state.Status) {
			member.Status = StartReadinessActive
			analysis.ActiveCount++
			analysis.Members = append(analysis.Members, member)
			continue
		}

		member.Warnings = append(member.Warnings, profileStartWarnings(profile)...)

		if err := s.commandChecker.CheckCommand(commandForProfile(profile)); err != nil {
			member.Problems = append(member.Problems, err.Error())
		}

		if err := validateProfileStart(profile); err != nil {
			member.Problems = append(member.Problems, err.Error())
		}

		if localPort, managed := managedLocalPort(profile); managed {
			if owner, exists := reservedPorts[localPort]; exists && owner != profile.Name {
				member.Problems = append(
					member.Problems,
					fmt.Sprintf("local port %d is already reserved by profile %q", localPort, owner),
				)
			} else if err := s.portChecker.CheckLocalPort(localPort); err != nil {
				member.Problems = append(member.Problems, err.Error())
			}
		}

		if len(member.Problems) > 0 {
			member.Status = StartReadinessBlocked
			analysis.BlockedCount++
			analysis.Members = append(analysis.Members, member)
			continue
		}

		if len(member.Warnings) > 0 {
			member.Status = StartReadinessWarning
			analysis.WarningCount++
			if localPort, managed := managedLocalPort(profile); managed {
				reservedPorts[localPort] = profile.Name
			}
			analysis.Members = append(analysis.Members, member)
			continue
		}

		member.Status = StartReadinessReady
		analysis.ReadyCount++
		if localPort, managed := managedLocalPort(profile); managed {
			reservedPorts[localPort] = profile.Name
		}
		analysis.Members = append(analysis.Members, member)
	}

	return analysis, nil
}

func (s *Service) RemoveProfile(name string, removeFromStacks bool, persist ConfigPersister) (RemoveProfileResult, error) {
	if _, err := s.profile(name); err != nil {
		return RemoveProfileResult{}, err
	}

	result := RemoveProfileResult{
		Name:              name,
		ReferencingStacks: s.config.StacksReferencingProfile(name),
	}

	if len(result.ReferencingStacks) > 0 && !removeFromStacks {
		return RemoveProfileResult{}, fmt.Errorf(
			"profile %q is still referenced by stacks: %s",
			name,
			strings.Join(result.ReferencingStacks, ", "),
		)
	}

	if state, exists := s.supervisor.Snapshot(name); exists && isActiveStatus(state.Status) {
		if err := s.supervisor.Stop(name); err != nil {
			return RemoveProfileResult{}, fmt.Errorf("stop profile %q before delete: %w", name, err)
		}
		result.WasActive = true
	}

	updatedConfig := cloneConfig(s.config)
	if !updatedConfig.RemoveProfile(name) {
		return RemoveProfileResult{}, fmt.Errorf("profile %q not found", name)
	}

	if removeFromStacks {
		result.UpdatedStacks, result.RemovedStacks = updatedConfig.RemoveProfileFromStacks(name)
	}

	if persist != nil {
		if err := persist(updatedConfig); err != nil {
			return RemoveProfileResult{}, fmt.Errorf("persist config after deleting profile %q: %w", name, err)
		}
	}

	s.applyConfig(updatedConfig)
	return result, nil
}

func (s *Service) RemoveStack(name string, persist ConfigPersister) (RemoveStackResult, error) {
	if _, err := s.stack(name); err != nil {
		return RemoveStackResult{}, err
	}

	updatedConfig := cloneConfig(s.config)
	if !updatedConfig.RemoveStack(name) {
		return RemoveStackResult{}, fmt.Errorf("stack %q not found", name)
	}

	if persist != nil {
		if err := persist(updatedConfig); err != nil {
			return RemoveStackResult{}, fmt.Errorf("persist config after deleting stack %q: %w", name, err)
		}
	}

	s.applyConfig(updatedConfig)
	return RemoveStackResult{Name: name}, nil
}

func (s *Service) Subscribe(buffer int) (int, <-chan ltruntime.Event) {
	return s.supervisor.Subscribe(buffer)
}

func (s *Service) Unsubscribe(id int) {
	s.supervisor.Unsubscribe(id)
}

func (s *Service) Config() domain.Config {
	return cloneConfig(s.config)
}

func (s *Service) ReplaceConfig(config domain.Config) {
	s.applyConfig(config)
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

func (s *Service) applyConfig(config domain.Config) {
	s.config = cloneConfig(config)

	s.profiles = make(map[string]domain.Profile, len(config.Profiles))
	for _, profile := range config.Profiles {
		s.profiles[profile.Name] = profile
	}
	s.profileList = append([]domain.Profile(nil), config.Profiles...)

	s.stacks = make(map[string]domain.Stack, len(config.Stacks))
	for _, stack := range config.Stacks {
		s.stacks[stack.Name] = stack
	}
	s.stackList = append([]domain.Stack(nil), config.Stacks...)
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

	if err := s.commandChecker.CheckCommand(commandForProfile(profile)); err != nil {
		return err
	}

	if err := validateProfileStart(profile); err != nil {
		return err
	}

	if localPort, managed := managedLocalPort(profile); managed {
		if owner, exists := s.activePortOwner(profile.Name, localPort); exists {
			return fmt.Errorf("local port %d is already used by active profile %q", localPort, owner)
		}

		if err := s.portChecker.CheckLocalPort(localPort); err != nil {
			return err
		}
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

		if localPort, managed := managedLocalPort(profile); managed {
			reservedPorts[localPort] = profileName
		}
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

		if err := s.commandChecker.CheckCommand(commandForProfile(profile)); err != nil {
			errs = append(errs, fmt.Errorf("profile %q: %w", profile.Name, err))
			continue
		}

		if err := validateProfileStart(profile); err != nil {
			errs = append(errs, fmt.Errorf("profile %q: %w", profile.Name, err))
			continue
		}

		if localPort, managed := managedLocalPort(profile); managed {
			if owner, exists := reservedPorts[localPort]; exists && owner != profile.Name {
				errs = append(errs, fmt.Errorf("profile %q: local port %d is already reserved by profile %q", profile.Name, localPort, owner))
				continue
			}

			if err := s.portChecker.CheckLocalPort(localPort); err != nil {
				errs = append(errs, fmt.Errorf("profile %q: %w", profile.Name, err))
				continue
			}

			reservedPorts[localPort] = profile.Name
		}
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

		localPort, managed := managedLocalPort(profile)
		if managed && localPort == port {
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

func validateProfileStart(profile domain.Profile) error {
	_, err := BuildProcessSpec(profile)
	return err
}

func commandForProfile(profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeKubernetesPortForward:
		return "kubectl"
	case domain.TunnelTypeSSHLocal, domain.TunnelTypeSSHRemote, domain.TunnelTypeSSHDynamic:
		return "ssh"
	default:
		return ""
	}
}

func profileStartWarnings(profile domain.Profile) []string {
	warnings := make([]string, 0, 4)

	if hasLabel(profile.Labels, "draft") {
		warnings = append(warnings, fmt.Sprintf("profile %q is still marked as draft", profile.Name))
	}

	switch profile.Type {
	case domain.TunnelTypeKubernetesPortForward:
		if profile.Kubernetes != nil {
			if strings.TrimSpace(profile.Kubernetes.Context) == "" {
				warnings = append(warnings, "kubernetes context is empty; will use the current kubectl context")
			}
			if strings.TrimSpace(profile.Kubernetes.Namespace) == "" {
				warnings = append(warnings, "namespace is empty; will use the context default")
			}
		}
	case domain.TunnelTypeSSHRemote:
		if profile.SSHRemote != nil && strings.TrimSpace(profile.SSHRemote.BindAddress) == "0.0.0.0" {
			warnings = append(warnings, "remote bind address 0.0.0.0 may expose this forward publicly")
		}
	case domain.TunnelTypeSSHDynamic:
		if profile.SSHDynamic != nil {
			bindAddress := strings.TrimSpace(profile.SSHDynamic.BindAddress)
			if bindAddress != "" && bindAddress != "127.0.0.1" && !strings.EqualFold(bindAddress, "localhost") {
				warnings = append(warnings, fmt.Sprintf("SOCKS bind address %q is not loopback; other machines may reach this proxy", bindAddress))
			}
		}
	}

	return warnings
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

func managedLocalPort(profile domain.Profile) (int, bool) {
	switch profile.Type {
	case domain.TunnelTypeSSHLocal, domain.TunnelTypeSSHDynamic, domain.TunnelTypeKubernetesPortForward:
		return profile.LocalPort, true
	default:
		return 0, false
	}
}

func cloneConfig(config domain.Config) domain.Config {
	cloned := domain.Config{
		Version:  config.Version,
		Language: config.Language,
		Profiles: make([]domain.Profile, 0, len(config.Profiles)),
		Stacks:   make([]domain.Stack, 0, len(config.Stacks)),
	}

	for _, profile := range config.Profiles {
		profileCopy := profile
		profileCopy.Labels = append([]string(nil), profile.Labels...)
		if profile.SSH != nil {
			sshCopy := *profile.SSH
			profileCopy.SSH = &sshCopy
		}
		if profile.SSHRemote != nil {
			sshRemoteCopy := *profile.SSHRemote
			profileCopy.SSHRemote = &sshRemoteCopy
		}
		if profile.SSHDynamic != nil {
			sshDynamicCopy := *profile.SSHDynamic
			profileCopy.SSHDynamic = &sshDynamicCopy
		}
		if profile.Kubernetes != nil {
			kubernetesCopy := *profile.Kubernetes
			profileCopy.Kubernetes = &kubernetesCopy
		}
		cloned.Profiles = append(cloned.Profiles, profileCopy)
	}

	for _, stack := range config.Stacks {
		stackCopy := stack
		stackCopy.Labels = append([]string(nil), stack.Labels...)
		stackCopy.Profiles = append([]string(nil), stack.Profiles...)
		cloned.Stacks = append(cloned.Stacks, stackCopy)
	}

	return cloned
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

type noopCommandChecker struct{}

func (noopCommandChecker) CheckCommand(string) error {
	return nil
}

type systemCommandChecker struct{}

func (systemCommandChecker) CheckCommand(command string) error {
	if strings.TrimSpace(command) == "" {
		return nil
	}
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", command)
	}
	return nil
}

func hasLabel(labels []string, label string) bool {
	for _, existing := range labels {
		if existing == label {
			return true
		}
	}
	return false
}
