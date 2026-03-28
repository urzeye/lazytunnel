package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
)

func TestServiceStartProfileBuildsAndStartsSpec(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.StartProfile("prod-db"); err != nil {
		t.Fatalf("start profile: %v", err)
	}

	if got := runtime.startedSpecs[0].Command; got != "ssh" {
		t.Fatalf("expected ssh command, got %q", got)
	}
}

func TestServiceStartProfileRejectsUnavailablePort(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(
		testConfig(),
		runtime,
		WithPortChecker(fakePortChecker{errs: map[int]error{
			5432: errors.New("address already in use"),
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.StartProfile("prod-db")
	if err == nil {
		t.Fatal("expected port conflict error")
	}

	if !strings.Contains(err.Error(), "address already in use") {
		t.Fatalf("expected address-in-use error, got %v", err)
	}
}

func TestServiceStartProfileRejectsManagedPortConflict(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	runtime.statesByName["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.StartProfile("redis-debug")
	if err == nil {
		t.Fatal("expected managed port conflict error")
	}

	if !strings.Contains(err.Error(), `local port 5432 is already used by active profile "prod-db"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceToggleProfileStopsActiveTunnel(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	runtime.statesByName["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.ToggleProfile("prod-db"); err != nil {
		t.Fatalf("toggle profile: %v", err)
	}

	if len(runtime.stoppedNames) != 1 || runtime.stoppedNames[0] != "prod-db" {
		t.Fatalf("expected stop call for prod-db, got %#v", runtime.stoppedNames)
	}
}

func TestServiceAnalyzeProfileStartReportsConfigProblems(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Profiles = append(cfg.Profiles, domain.Profile{
		Name:      "broken-ssh",
		Type:      domain.TunnelTypeSSHLocal,
		LocalPort: 15432,
	})

	service, err := NewService(cfg, newFakeRuntimeController(), WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeProfileStart("broken-ssh")
	if err != nil {
		t.Fatalf("analyze profile: %v", err)
	}

	if analysis.Status != StartReadinessBlocked {
		t.Fatalf("expected blocked status, got %q", analysis.Status)
	}
	if len(analysis.Problems) == 0 || !strings.Contains(analysis.Problems[0], "ssh settings are required") {
		t.Fatalf("expected ssh settings problem, got %#v", analysis.Problems)
	}
}

func TestServiceAnalyzeProfileStartReportsManagedPortConflict(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	runtime.statesByName["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeProfileStart("redis-debug")
	if err != nil {
		t.Fatalf("analyze profile: %v", err)
	}

	if analysis.Status != StartReadinessBlocked {
		t.Fatalf("expected blocked status, got %q", analysis.Status)
	}
	if len(analysis.Problems) == 0 || !strings.Contains(analysis.Problems[0], `local port 5432 is already used by active profile "prod-db"`) {
		t.Fatalf("expected managed port conflict, got %#v", analysis.Problems)
	}
}

func TestServiceProfileViewsIncludesDefaultStoppedState(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	views := service.ProfileViews()
	if len(views) != 3 {
		t.Fatalf("expected 3 views, got %d", len(views))
	}

	if got := views[0].State.Status; got != domain.TunnelStatusStopped {
		t.Fatalf("expected default stopped state, got %q", got)
	}
}

func TestServiceStackViewsExposePartialStatus(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	runtime.statesByName["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	views := service.StackViews()
	if len(views) != 1 {
		t.Fatalf("expected 1 stack view, got %d", len(views))
	}

	if got := views[0].Status; got != StackStatusPartial {
		t.Fatalf("expected partial stack status, got %q", got)
	}
}

func TestServiceAnalyzeStackStartReportsReadyActiveAndBlockedMembers(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Stacks = []domain.Stack{
		{
			Name:     "mixed-dev",
			Profiles: []string{"prod-db", "api-debug", "redis-debug", "missing-profile"},
		},
	}

	runtime := newFakeRuntimeController()
	runtime.statesByName["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := NewService(cfg, runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeStackStart("mixed-dev")
	if err != nil {
		t.Fatalf("analyze stack: %v", err)
	}

	if analysis.ActiveCount != 1 || analysis.ReadyCount != 1 || analysis.BlockedCount != 2 {
		t.Fatalf("unexpected analysis counts: %#v", analysis)
	}
	if got := analysis.Members[0].Status; got != StartReadinessActive {
		t.Fatalf("expected prod-db active, got %q", got)
	}
	if got := analysis.Members[1].Status; got != StartReadinessReady {
		t.Fatalf("expected api-debug ready, got %q", got)
	}
	if got := analysis.Members[2].Status; got != StartReadinessBlocked {
		t.Fatalf("expected redis-debug blocked, got %q", got)
	}
	if len(analysis.Members[2].Problems) == 0 || !strings.Contains(analysis.Members[2].Problems[0], `local port 5432 is already reserved by profile "prod-db"`) {
		t.Fatalf("expected redis-debug reserved-port problem, got %#v", analysis.Members[2].Problems)
	}
	if got := analysis.Members[3].Status; got != StartReadinessBlocked {
		t.Fatalf("expected missing-profile blocked, got %q", got)
	}
	if len(analysis.Members[3].Problems) == 0 || !strings.Contains(analysis.Members[3].Problems[0], `profile "missing-profile" not found`) {
		t.Fatalf("expected missing-profile error, got %#v", analysis.Members[3].Problems)
	}
}

func TestServiceStartStackStartsInactiveMembers(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.StartStack("backend-dev"); err != nil {
		t.Fatalf("start stack: %v", err)
	}

	if got := len(runtime.startedSpecs); got != 2 {
		t.Fatalf("expected 2 started specs, got %d", got)
	}
}

func TestServiceStartStackRejectsPortConflictBeforeStart(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(
		testConfig(),
		runtime,
		WithPortChecker(fakePortChecker{errs: map[int]error{
			8080: errors.New("address already in use"),
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.StartStack("backend-dev")
	if err == nil {
		t.Fatal("expected stack preflight error")
	}

	if got := len(runtime.startedSpecs); got != 0 {
		t.Fatalf("expected no started specs on failed preflight, got %d", got)
	}
}

func TestServiceToggleStackStopsWhenFullyRunning(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	runtime.statesByName["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}
	runtime.statesByName["api-debug"] = domain.RuntimeState{
		ProfileName: "api-debug",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.ToggleStack("backend-dev"); err != nil {
		t.Fatalf("toggle stack: %v", err)
	}

	if len(runtime.stoppedNames) != 2 {
		t.Fatalf("expected 2 stop calls, got %#v", runtime.stoppedNames)
	}
}

func TestServiceRemoveProfilePrunesStackReferencesAndStopsActiveTunnel(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	runtime.statesByName["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
	}

	service, err := NewService(testConfig(), runtime, WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var persisted domain.Config
	result, err := service.RemoveProfile("prod-db", true, func(cfg domain.Config) error {
		persisted = cfg
		return nil
	})
	if err != nil {
		t.Fatalf("remove profile: %v", err)
	}

	if !result.WasActive {
		t.Fatal("expected active profile removal to report WasActive")
	}
	if result.UpdatedStacks != 1 {
		t.Fatalf("expected 1 updated stack, got %d", result.UpdatedStacks)
	}
	if result.RemovedStacks != 0 {
		t.Fatalf("expected 0 removed stacks, got %d", result.RemovedStacks)
	}
	if len(runtime.stoppedNames) != 1 || runtime.stoppedNames[0] != "prod-db" {
		t.Fatalf("expected stop call for prod-db, got %#v", runtime.stoppedNames)
	}

	if got := len(service.Profiles()); got != 2 {
		t.Fatalf("expected 2 remaining profiles, got %d", got)
	}
	if got := len(service.Stacks()); got != 1 {
		t.Fatalf("expected 1 remaining stack, got %d", got)
	}
	if want := []string{"api-debug"}; strings.Join(service.Stacks()[0].Profiles, ",") != strings.Join(want, ",") {
		t.Fatalf("expected remaining stack members %v, got %v", want, service.Stacks()[0].Profiles)
	}
	if got := len(persisted.Profiles); got != 2 {
		t.Fatalf("expected persisted config to have 2 profiles, got %d", got)
	}
}

func TestServiceRemoveProfileRejectsReferencedProfileWithoutPrune(t *testing.T) {
	t.Parallel()

	service, err := NewService(testConfig(), newFakeRuntimeController(), WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = service.RemoveProfile("prod-db", false, nil)
	if err == nil {
		t.Fatal("expected remove profile to fail")
	}

	if !strings.Contains(err.Error(), "still referenced by stacks") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceRemoveStackUpdatesInMemoryConfig(t *testing.T) {
	t.Parallel()

	service, err := NewService(testConfig(), newFakeRuntimeController(), WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = service.RemoveStack("backend-dev", nil)
	if err != nil {
		t.Fatalf("remove stack: %v", err)
	}

	if got := len(service.Stacks()); got != 0 {
		t.Fatalf("expected no stacks, got %d", got)
	}
}

func testConfig() domain.Config {
	return domain.Config{
		Version: 1,
		Profiles: []domain.Profile{
			{
				Name:        "prod-db",
				Description: "Database tunnel",
				Type:        domain.TunnelTypeSSHLocal,
				LocalPort:   5432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
			{
				Name:      "api-debug",
				Type:      domain.TunnelTypeKubernetesPortForward,
				LocalPort: 8080,
				Kubernetes: &domain.Kubernetes{
					Context:      "dev-cluster",
					Namespace:    "backend",
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
			{
				Name:      "redis-debug",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 5432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "redis.internal",
					RemotePort: 6379,
				},
			},
		},
		Stacks: []domain.Stack{
			{
				Name:     "backend-dev",
				Profiles: []string{"prod-db", "api-debug"},
			},
		},
	}
}

type fakeRuntimeController struct {
	startedSpecs []ltruntime.ProcessSpec
	stoppedNames []string
	statesByName map[string]domain.RuntimeState
}

func newFakeRuntimeController() *fakeRuntimeController {
	return &fakeRuntimeController{
		statesByName: make(map[string]domain.RuntimeState),
	}
}

func (f *fakeRuntimeController) Start(spec ltruntime.ProcessSpec) error {
	f.startedSpecs = append(f.startedSpecs, spec)
	f.statesByName[spec.Name] = domain.RuntimeState{
		ProfileName: spec.Name,
		Status:      domain.TunnelStatusRunning,
		PID:         1,
	}
	return nil
}

func (f *fakeRuntimeController) Stop(name string) error {
	f.stoppedNames = append(f.stoppedNames, name)
	f.statesByName[name] = domain.RuntimeState{
		ProfileName: name,
		Status:      domain.TunnelStatusStopped,
	}
	return nil
}

func (f *fakeRuntimeController) Snapshot(name string) (domain.RuntimeState, bool) {
	state, ok := f.statesByName[name]
	return state, ok
}

func (f *fakeRuntimeController) ListStates() []domain.RuntimeState {
	states := make([]domain.RuntimeState, 0, len(f.statesByName))
	for _, state := range f.statesByName {
		states = append(states, state)
	}
	return states
}

func (f *fakeRuntimeController) Subscribe(buffer int) (int, <-chan ltruntime.Event) {
	ch := make(chan ltruntime.Event)
	return 1, ch
}

func (f *fakeRuntimeController) Unsubscribe(id int) {}

type fakePortChecker struct {
	errs map[int]error
}

func (f fakePortChecker) CheckLocalPort(port int) error {
	if err, exists := f.errs[port]; exists {
		return err
	}

	return nil
}
