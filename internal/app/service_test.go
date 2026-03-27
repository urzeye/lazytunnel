package app

import (
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
)

func TestServiceStartProfileBuildsAndStartsSpec(t *testing.T) {
	t.Parallel()

	runtime := &fakeRuntimeController{}
	service, err := NewService(domain.Config{
		Version: 1,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 5432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}, runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.StartProfile("prod-db"); err != nil {
		t.Fatalf("start profile: %v", err)
	}

	if got := runtime.started.Command; got != "ssh" {
		t.Fatalf("expected ssh command, got %q", got)
	}
}

func TestServiceToggleProfileStopsActiveTunnel(t *testing.T) {
	t.Parallel()

	runtime := &fakeRuntimeController{
		snapshots: map[string]domain.RuntimeState{
			"prod-db": {
				ProfileName: "prod-db",
				Status:      domain.TunnelStatusRunning,
			},
		},
	}

	service, err := NewService(domain.Config{
		Version: 1,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 5432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}, runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.ToggleProfile("prod-db"); err != nil {
		t.Fatalf("toggle profile: %v", err)
	}

	if runtime.stoppedName != "prod-db" {
		t.Fatalf("expected stop call for prod-db, got %q", runtime.stoppedName)
	}
}

func TestServiceProfileViewsIncludesDefaultStoppedState(t *testing.T) {
	t.Parallel()

	runtime := &fakeRuntimeController{}
	service, err := NewService(domain.Config{
		Version: 1,
		Profiles: []domain.Profile{
			{
				Name:      "api-debug",
				Type:      domain.TunnelTypeKubernetesPortForward,
				LocalPort: 8080,
				Kubernetes: &domain.Kubernetes{
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
		},
	}, runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	views := service.ProfileViews()
	if len(views) != 1 {
		t.Fatalf("expected 1 view, got %d", len(views))
	}

	if got := views[0].State.Status; got != domain.TunnelStatusStopped {
		t.Fatalf("expected default stopped state, got %q", got)
	}
}

type fakeRuntimeController struct {
	started     ltruntime.ProcessSpec
	stoppedName string
	snapshots   map[string]domain.RuntimeState
	states      []domain.RuntimeState
}

func (f *fakeRuntimeController) Start(spec ltruntime.ProcessSpec) error {
	f.started = spec
	return nil
}

func (f *fakeRuntimeController) Stop(name string) error {
	f.stoppedName = name
	return nil
}

func (f *fakeRuntimeController) Snapshot(name string) (domain.RuntimeState, bool) {
	state, ok := f.snapshots[name]
	return state, ok
}

func (f *fakeRuntimeController) ListStates() []domain.RuntimeState {
	return append([]domain.RuntimeState(nil), f.states...)
}

func (f *fakeRuntimeController) Subscribe(buffer int) (int, <-chan ltruntime.Event) {
	ch := make(chan ltruntime.Event)
	return 1, ch
}

func (f *fakeRuntimeController) Unsubscribe(id int) {}
