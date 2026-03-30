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

func TestServiceStartProfileSkipsLocalPortChecksForSSHRemote(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Profiles = append(cfg.Profiles, domain.Profile{
		Name: "public-api",
		Type: domain.TunnelTypeSSHRemote,
		SSHRemote: &domain.SSHRemote{
			Host:       "bastion-prod",
			BindPort:   9000,
			TargetHost: "127.0.0.1",
			TargetPort: 8080,
		},
	})

	service, err := NewService(
		cfg,
		newFakeRuntimeController(),
		WithPortChecker(fakePortChecker{errs: map[int]error{
			9000: errors.New("address already in use"),
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	if err := service.StartProfile("public-api"); err != nil {
		t.Fatalf("expected ssh remote profile to skip local port checks, got %v", err)
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

func TestServiceRestartProfileStopsThenStartsTunnel(t *testing.T) {
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

	if err := service.RestartProfile("prod-db"); err != nil {
		t.Fatalf("restart profile: %v", err)
	}

	if len(runtime.stoppedNames) != 1 || runtime.stoppedNames[0] != "prod-db" {
		t.Fatalf("expected stop call for prod-db, got %#v", runtime.stoppedNames)
	}
	if len(runtime.startedSpecs) != 1 || runtime.startedSpecs[0].Name != "prod-db" {
		t.Fatalf("expected restart start call for prod-db, got %#v", runtime.startedSpecs)
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

func TestServiceAnalyzeProfileStartReportsUnavailableLocalPort(t *testing.T) {
	t.Parallel()

	service, err := NewService(
		testConfig(),
		newFakeRuntimeController(),
		WithPortChecker(fakePortChecker{errs: map[int]error{
			8080: errors.New("local port 8080 is unavailable: address already in use"),
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeProfileStart("api-debug")
	if err != nil {
		t.Fatalf("analyze profile: %v", err)
	}

	if analysis.Status != StartReadinessBlocked {
		t.Fatalf("expected blocked status, got %q", analysis.Status)
	}
	if len(analysis.Problems) == 0 || !strings.Contains(analysis.Problems[0], "local port 8080 is unavailable") {
		t.Fatalf("expected unavailable-port problem, got %#v", analysis.Problems)
	}
}

func TestServiceAnalyzeProfileStartReportsWarnings(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "draft-api",
				Type:      domain.TunnelTypeKubernetesPortForward,
				LocalPort: 8080,
				Labels:    []string{"draft"},
				Kubernetes: &domain.Kubernetes{
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
		},
	}

	service, err := NewService(cfg, newFakeRuntimeController(), WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeProfileStart("draft-api")
	if err != nil {
		t.Fatalf("analyze profile: %v", err)
	}

	if analysis.Status != StartReadinessWarning {
		t.Fatalf("expected warning status, got %q", analysis.Status)
	}
	if len(analysis.Warnings) < 2 {
		t.Fatalf("expected multiple warnings, got %#v", analysis.Warnings)
	}
	if !strings.Contains(strings.Join(analysis.Warnings, "\n"), "still marked as draft") {
		t.Fatalf("expected draft warning, got %#v", analysis.Warnings)
	}
	if !strings.Contains(strings.Join(analysis.Warnings, "\n"), "current kubectl context") {
		t.Fatalf("expected current-context warning, got %#v", analysis.Warnings)
	}
}

func TestServiceAnalyzeProfileStartReportsMissingCommand(t *testing.T) {
	t.Parallel()

	service, err := NewService(
		testConfig(),
		newFakeRuntimeController(),
		WithPortChecker(fakePortChecker{}),
		WithCommandChecker(fakeCommandChecker{errs: map[string]error{
			"kubectl": errors.New("kubectl is not installed or not in PATH"),
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeProfileStart("api-debug")
	if err != nil {
		t.Fatalf("analyze profile: %v", err)
	}

	if analysis.Status != StartReadinessBlocked {
		t.Fatalf("expected blocked status, got %q", analysis.Status)
	}
	if len(analysis.Problems) == 0 || !strings.Contains(analysis.Problems[0], "kubectl is not installed or not in PATH") {
		t.Fatalf("expected missing-command problem, got %#v", analysis.Problems)
	}
}

func TestServiceAnalyzeProfileStartIncludesProbeWarnings(t *testing.T) {
	t.Parallel()

	service, err := NewService(
		testConfig(),
		newFakeRuntimeController(),
		WithPortChecker(fakePortChecker{}),
		WithProfileProbeChecker(fakeProfileProbeChecker{results: map[string]ProfileProbeResult{
			"prod-db": {
				Warnings: []string{`SSH host alias "bastion-prod" was not found in ~/.ssh/config; ssh will treat it as a raw hostname`},
			},
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeProfileStart("prod-db")
	if err != nil {
		t.Fatalf("analyze profile: %v", err)
	}

	if analysis.Status != StartReadinessWarning {
		t.Fatalf("expected warning status, got %q", analysis.Status)
	}
	if len(analysis.Warnings) == 0 || !strings.Contains(analysis.Warnings[0], "SSH host alias") {
		t.Fatalf("expected probe warning, got %#v", analysis.Warnings)
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

func TestServiceAnalyzeStackStartCountsWarningMembers(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "draft-api",
				Type:      domain.TunnelTypeKubernetesPortForward,
				LocalPort: 8080,
				Labels:    []string{"draft"},
				Kubernetes: &domain.Kubernetes{
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
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
		Stacks: []domain.Stack{
			{
				Name:     "warning-dev",
				Profiles: []string{"draft-api", "prod-db"},
			},
		},
	}

	service, err := NewService(cfg, newFakeRuntimeController(), WithPortChecker(fakePortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeStackStart("warning-dev")
	if err != nil {
		t.Fatalf("analyze stack: %v", err)
	}

	if analysis.WarningCount != 1 || analysis.ReadyCount != 1 || analysis.BlockedCount != 0 {
		t.Fatalf("unexpected warning analysis counts: %#v", analysis)
	}
	if got := analysis.Members[0].Status; got != StartReadinessWarning {
		t.Fatalf("expected draft-api warning, got %q", got)
	}
	if len(analysis.Members[0].Warnings) == 0 || !strings.Contains(analysis.Members[0].Warnings[0], "still marked as draft") {
		t.Fatalf("expected draft warning, got %#v", analysis.Members[0].Warnings)
	}
}

func TestServiceAnalyzeStackStartIncludesProbeProblems(t *testing.T) {
	t.Parallel()

	service, err := NewService(
		testConfig(),
		newFakeRuntimeController(),
		WithPortChecker(fakePortChecker{}),
		WithProfileProbeChecker(fakeProfileProbeChecker{results: map[string]ProfileProbeResult{
			"api-debug": {
				Problems: []string{`kubernetes service "api" was not found in namespace "backend" for context "dev-cluster"`},
			},
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	analysis, err := service.AnalyzeStackStart("backend-dev")
	if err != nil {
		t.Fatalf("analyze stack: %v", err)
	}

	if analysis.BlockedCount != 1 {
		t.Fatalf("expected one blocked member, got %#v", analysis)
	}
	if got := analysis.Members[1].Status; got != StartReadinessBlocked {
		t.Fatalf("expected api-debug blocked, got %q", got)
	}
	if len(analysis.Members[1].Problems) == 0 || !strings.Contains(analysis.Members[1].Problems[0], `kubernetes service "api" was not found`) {
		t.Fatalf("expected probe problem, got %#v", analysis.Members[1].Problems)
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

func TestServiceStartProfileRejectsMissingCommandBeforeStart(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(
		testConfig(),
		runtime,
		WithPortChecker(fakePortChecker{}),
		WithCommandChecker(fakeCommandChecker{errs: map[string]error{
			"kubectl": errors.New("kubectl is not installed or not in PATH"),
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.StartProfile("api-debug")
	if err == nil {
		t.Fatal("expected missing-command error")
	}
	if !strings.Contains(err.Error(), "kubectl is not installed or not in PATH") {
		t.Fatalf("expected missing-command error, got %v", err)
	}
	if got := len(runtime.startedSpecs); got != 0 {
		t.Fatalf("expected no started specs on failed start, got %d", got)
	}
}

func TestServiceStartStackRejectsMissingCommandBeforeStart(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(
		testConfig(),
		runtime,
		WithPortChecker(fakePortChecker{}),
		WithCommandChecker(fakeCommandChecker{errs: map[string]error{
			"kubectl": errors.New("kubectl is not installed or not in PATH"),
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.StartStack("backend-dev")
	if err == nil {
		t.Fatal("expected missing-command preflight error")
	}
	if !strings.Contains(err.Error(), `profile "api-debug": kubectl is not installed or not in PATH`) {
		t.Fatalf("expected missing-command preflight error, got %v", err)
	}
	if got := len(runtime.startedSpecs); got != 0 {
		t.Fatalf("expected no started specs on failed preflight, got %d", got)
	}
}

func TestServiceStartProfileRejectsProbeProblemsBeforeStart(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntimeController()
	service, err := NewService(
		testConfig(),
		runtime,
		WithPortChecker(fakePortChecker{}),
		WithProfileProbeChecker(fakeProfileProbeChecker{results: map[string]ProfileProbeResult{
			"api-debug": {
				Problems: []string{`kubernetes service "api" was not found in namespace "backend" for context "dev-cluster"`},
			},
		}}),
	)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	err = service.StartProfile("api-debug")
	if err == nil {
		t.Fatal("expected probe blocker error")
	}
	if !strings.Contains(err.Error(), `kubernetes service "api" was not found`) {
		t.Fatalf("expected probe blocker error, got %v", err)
	}
	if got := len(runtime.startedSpecs); got != 0 {
		t.Fatalf("expected no started specs on failed start, got %d", got)
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

func TestServiceRestartStackStopsMembersThenStartsThemAgain(t *testing.T) {
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

	if err := service.RestartStack("backend-dev"); err != nil {
		t.Fatalf("restart stack: %v", err)
	}

	if len(runtime.stoppedNames) != 2 {
		t.Fatalf("expected 2 stop calls, got %#v", runtime.stoppedNames)
	}
	if len(runtime.startedSpecs) != 2 {
		t.Fatalf("expected 2 restarted specs, got %#v", runtime.startedSpecs)
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

type fakeCommandChecker struct {
	errs map[string]error
}

func (f fakeCommandChecker) CheckCommand(command string) error {
	if err, exists := f.errs[command]; exists {
		return err
	}
	return nil
}

type fakeProfileProbeChecker struct {
	results map[string]ProfileProbeResult
}

func (f fakeProfileProbeChecker) CheckProfile(profile domain.Profile, force bool) ProfileProbeResult {
	if result, exists := f.results[profile.Name]; exists {
		return result
	}
	return ProfileProbeResult{}
}
