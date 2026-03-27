package app

import (
	"fmt"

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

type Service struct {
	config      domain.Config
	supervisor  RuntimeController
	profiles    map[string]domain.Profile
	profileList []domain.Profile
}

type ProfileView struct {
	Profile domain.Profile
	State   domain.RuntimeState
}

func NewService(config domain.Config, supervisor RuntimeController) (*Service, error) {
	profiles := make(map[string]domain.Profile, len(config.Profiles))
	for _, profile := range config.Profiles {
		profiles[profile.Name] = profile
	}

	return &Service{
		config:      config,
		supervisor:  supervisor,
		profiles:    profiles,
		profileList: append([]domain.Profile(nil), config.Profiles...),
	}, nil
}

func (s *Service) Profiles() []domain.Profile {
	return append([]domain.Profile(nil), s.profileList...)
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
			state = domain.RuntimeState{
				ProfileName: profile.Name,
				Status:      domain.TunnelStatusStopped,
			}
		}

		views = append(views, ProfileView{
			Profile: profile,
			State:   state,
		})
	}

	return views
}

func (s *Service) StartProfile(name string) error {
	profile, err := s.profile(name)
	if err != nil {
		return err
	}

	spec, err := BuildProcessSpec(profile)
	if err != nil {
		return err
	}

	return s.supervisor.Start(spec)
}

func (s *Service) StopProfile(name string) error {
	if _, err := s.profile(name); err != nil {
		return err
	}

	return s.supervisor.Stop(name)
}

func (s *Service) ToggleProfile(name string) error {
	if state, exists := s.supervisor.Snapshot(name); exists && isActiveStatus(state.Status) {
		return s.supervisor.Stop(name)
	}

	return s.StartProfile(name)
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

func isActiveStatus(status domain.TunnelStatus) bool {
	switch status {
	case domain.TunnelStatusStarting, domain.TunnelStatusRunning, domain.TunnelStatusRestarting:
		return true
	default:
		return false
	}
}
