package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
	"github.com/urzeye/lazytunnel/internal/storage"
)

func TestProfileTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		profile domain.Profile
		want    string
	}{
		{
			name: "ssh local",
			profile: domain.Profile{
				Type: domain.TunnelTypeSSHLocal,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
			want: "bastion-prod -> db.internal:5432",
		},
		{
			name: "kubernetes",
			profile: domain.Profile{
				Type: domain.TunnelTypeKubernetesPortForward,
				Kubernetes: &domain.Kubernetes{
					Context:      "dev-cluster",
					Namespace:    "backend",
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
			want: "dev-cluster • backend • service/api:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := profileTarget(tt.profile); got != tt.want {
				t.Fatalf("profileTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecentStackActivitySortsAndLimits(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC)
	view := app.StackView{
		Members: []app.ProfileView{
			{
				Profile: domain.Profile{Name: "prod-db"},
				State: domain.RuntimeState{
					RecentLogs: []domain.LogEntry{
						{Timestamp: base.Add(3 * time.Second), Source: domain.LogSourceSystem, Message: "db third"},
						{Timestamp: base.Add(1 * time.Second), Source: domain.LogSourceSystem, Message: "db first"},
					},
				},
			},
			{
				Profile: domain.Profile{Name: "api-debug"},
				State: domain.RuntimeState{
					RecentLogs: []domain.LogEntry{
						{Timestamp: base.Add(2 * time.Second), Source: domain.LogSourceStdout, Message: "api second"},
					},
				},
			},
		},
	}

	got := recentStackActivity(view, 2)
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(got))
	}

	if got[0].ProfileName != "api-debug" || got[0].Log.Message != "api second" {
		t.Fatalf("unexpected first entry: %#v", got[0])
	}

	if got[1].ProfileName != "prod-db" || got[1].Log.Message != "db third" {
		t.Fatalf("unexpected second entry: %#v", got[1])
	}
}

func TestFormatLastExit(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC)
	exitedAt := now.Add(-42 * time.Second)

	state := domain.RuntimeState{
		ExitedAt:     &exitedAt,
		ExitReason:   "stopped by user",
		LastExitCode: 0,
	}

	got := formatLastExit(state, now)
	want := "stopped by user • 42s ago • code 0"
	if got != want {
		t.Fatalf("formatLastExit() = %q, want %q", got, want)
	}
}

func TestFilterProfileViewsMatchesLabelsAndTargets(t *testing.T) {
	t.Parallel()

	views := []app.ProfileView{
		{
			Profile: domain.Profile{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 5432,
				Labels:    []string{"prod", "db"},
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
		{
			Profile: domain.Profile{
				Name:      "api-debug",
				Type:      domain.TunnelTypeKubernetesPortForward,
				LocalPort: 8080,
				Labels:    []string{"dev", "api"},
				Kubernetes: &domain.Kubernetes{
					Context:      "dev-cluster",
					Namespace:    "backend",
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
		},
	}

	if got := filterProfileViews(views, "prod"); len(got) != 1 || got[0].Profile.Name != "prod-db" {
		t.Fatalf("expected prod filter to match prod-db, got %#v", got)
	}

	if got := filterProfileViews(views, "service/api"); len(got) != 1 || got[0].Profile.Name != "api-debug" {
		t.Fatalf("expected target filter to match api-debug, got %#v", got)
	}
}

func TestFilterStackViewsMatchesMembersAndLabels(t *testing.T) {
	t.Parallel()

	views := []app.StackView{
		{
			Stack: domain.Stack{
				Name:   "backend-dev",
				Labels: []string{"daily"},
			},
			Members: []app.ProfileView{
				{Profile: domain.Profile{Name: "prod-db"}},
				{Profile: domain.Profile{Name: "api-debug"}},
			},
		},
		{
			Stack: domain.Stack{
				Name:   "ops",
				Labels: []string{"infra"},
			},
			Members: []app.ProfileView{
				{Profile: domain.Profile{Name: "grafana"}},
			},
		},
	}

	if got := filterStackViews(views, "api-debug"); len(got) != 1 || got[0].Stack.Name != "backend-dev" {
		t.Fatalf("expected member filter to match backend-dev, got %#v", got)
	}

	if got := filterStackViews(views, "infra"); len(got) != 1 || got[0].Stack.Name != "ops" {
		t.Fatalf("expected label filter to match ops, got %#v", got)
	}
}

func TestTrimLastWord(t *testing.T) {
	t.Parallel()

	if got := trimLastWord("prod db"); got != "prod" {
		t.Fatalf("trimLastWord() = %q, want %q", got, "prod")
	}

	if got := trimLastWord("single"); got != "" {
		t.Fatalf("trimLastWord() = %q, want empty string", got)
	}
}

func TestBuildDeleteRequestIncludesStackImpact(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")

	request := model.buildDeleteRequest(profiles, stacks)
	if request == nil {
		t.Fatal("expected delete request")
	}

	if !strings.Contains(request.Message, "stack references will be pruned") {
		t.Fatalf("expected stack pruning message, got %q", request.Message)
	}
}

func TestWindowAroundSelectionCentersWhenPossible(t *testing.T) {
	t.Parallel()

	start, end := windowAroundSelection(10, 5, 4)
	if start != 3 || end != 7 {
		t.Fatalf("windowAroundSelection() = (%d, %d), want (3, 7)", start, end)
	}
}

func TestWindowAroundSelectionPinsToEdges(t *testing.T) {
	t.Parallel()

	start, end := windowAroundSelection(10, 0, 4)
	if start != 0 || end != 4 {
		t.Fatalf("expected leading edge window, got (%d, %d)", start, end)
	}

	start, end = windowAroundSelection(10, 9, 4)
	if start != 6 || end != 10 {
		t.Fatalf("expected trailing edge window, got (%d, %d)", start, end)
	}
}

func TestClipLinesUsesOffsetAndLimit(t *testing.T) {
	t.Parallel()

	got := clipLines([]string{"a", "b", "c", "d"}, 1, 2)
	want := []string{"b", "c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("clipLines() = %#v, want %#v", got, want)
	}
}

func TestSplitListInspectorHeightsKeepsInspectorLarger(t *testing.T) {
	t.Parallel()

	listHeight, inspectorHeight := splitListInspectorHeights(18)
	if listHeight+inspectorHeight+1 != 18 {
		t.Fatalf("expected heights to fill total budget, got %d + %d + 1", listHeight, inspectorHeight)
	}
	if inspectorHeight <= listHeight {
		t.Fatalf("expected inspector to be larger than list, got list=%d inspector=%d", listHeight, inspectorHeight)
	}
}

func TestConfirmDeletePersistsProfileRemoval(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	runtime := newStubRuntimeController()
	runtime.states["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := app.NewService(storage.SampleConfig(), runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:       service,
		configPath:    configPath,
		pendingDelete: &deleteRequest{Kind: deleteKindProfile, Name: "prod-db"},
	}

	model = model.confirmDelete()

	if !strings.Contains(model.lastNotice, "Removed profile prod-db.") {
		t.Fatalf("expected success notice, got %q", model.lastNotice)
	}
	if len(runtime.stoppedNames) != 1 || runtime.stoppedNames[0] != "prod-db" {
		t.Fatalf("expected prod-db to be stopped before delete, got %#v", runtime.stoppedNames)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 1 {
		t.Fatalf("expected 1 profile after delete, got %d", got)
	}
	if got := cfg.Profiles[0].Name; got != "api-debug" {
		t.Fatalf("expected api-debug to remain, got %q", got)
	}
	if got := len(cfg.Stacks); got != 1 {
		t.Fatalf("expected 1 stack after prune, got %d", got)
	}
	if got := strings.Join(cfg.Stacks[0].Profiles, ","); got != "api-debug" {
		t.Fatalf("expected stack to be pruned to api-debug, got %q", got)
	}
}

type stubRuntimeController struct {
	states        map[string]domain.RuntimeState
	stoppedNames  []string
	subscriptions map[int]chan ltruntime.Event
}

func newStubRuntimeController() *stubRuntimeController {
	return &stubRuntimeController{
		states:        make(map[string]domain.RuntimeState),
		subscriptions: make(map[int]chan ltruntime.Event),
	}
}

func (s *stubRuntimeController) Start(spec ltruntime.ProcessSpec) error {
	s.states[spec.Name] = domain.RuntimeState{
		ProfileName: spec.Name,
		Status:      domain.TunnelStatusRunning,
		PID:         1,
	}
	return nil
}

func (s *stubRuntimeController) Stop(name string) error {
	s.stoppedNames = append(s.stoppedNames, name)
	s.states[name] = domain.RuntimeState{
		ProfileName: name,
		Status:      domain.TunnelStatusStopped,
	}
	return nil
}

func (s *stubRuntimeController) Snapshot(name string) (domain.RuntimeState, bool) {
	state, ok := s.states[name]
	return state, ok
}

func (s *stubRuntimeController) ListStates() []domain.RuntimeState {
	states := make([]domain.RuntimeState, 0, len(s.states))
	for _, state := range s.states {
		states = append(states, state)
	}
	return states
}

func (s *stubRuntimeController) Subscribe(buffer int) (int, <-chan ltruntime.Event) {
	id := len(s.subscriptions) + 1
	ch := make(chan ltruntime.Event, buffer)
	s.subscriptions[id] = ch
	return id, ch
}

func (s *stubRuntimeController) Unsubscribe(id int) {
	if ch, ok := s.subscriptions[id]; ok {
		delete(s.subscriptions, id)
		close(ch)
	}
}
