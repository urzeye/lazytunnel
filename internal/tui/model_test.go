package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
		{
			name: "ssh remote",
			profile: domain.Profile{
				Type: domain.TunnelTypeSSHRemote,
				SSHRemote: &domain.SSHRemote{
					Host:        "bastion-prod",
					BindAddress: "0.0.0.0",
					BindPort:    9000,
					TargetHost:  "127.0.0.1",
					TargetPort:  8080,
				},
			},
			want: "bastion-prod • remote 0.0.0.0:9000 -> 127.0.0.1:8080",
		},
		{
			name: "ssh dynamic",
			profile: domain.Profile{
				Type:      domain.TunnelTypeSSHDynamic,
				LocalPort: 1080,
				SSHDynamic: &domain.SSHDynamic{
					Host:        "bastion-prod",
					BindAddress: "127.0.0.1",
				},
			},
			want: "bastion-prod • SOCKS :1080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := profileTarget(domain.LanguageEnglish, tt.profile); got != tt.want {
				t.Fatalf("profileTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderProfileDetailLinesShowsConfigProblem(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "broken-ssh",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				Restart: domain.RestartPolicy{
					Enabled: true,
				},
			},
		},
	}, newStubRuntimeController(), app.WithPortChecker(stubPortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service: service,
		now:     time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC),
	}

	lines := model.renderProfileDetailLines(service.ProfileViews()[0], 80)

	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "Start") {
		t.Fatalf("expected start section, got %q", rendered)
	}
	if !strings.Contains(rendered, "Blocked") {
		t.Fatalf("expected blocked readiness, got %q", rendered)
	}
	if !strings.Contains(rendered, "Problem") {
		t.Fatalf("expected problem section, got %q", rendered)
	}
	if !strings.Contains(rendered, "ssh settings are required") {
		t.Fatalf("expected config validation message, got %q", rendered)
	}
	if !strings.Contains(rendered, "Fix") || !strings.Contains(rendered, "Press e") {
		t.Fatalf("expected actionable fix hint, got %q", rendered)
	}
}

func TestRenderStackDetailLinesShowsMissingProfiles(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
		Version: domain.CurrentConfigVersion,
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
		Stacks: []domain.Stack{
			{
				Name:     "backend-dev",
				Profiles: []string{"prod-db", "missing-api"},
			},
		},
	}, newStubRuntimeController(), app.WithPortChecker(stubPortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	lines := model.renderStackDetailLines(service.StackViews()[0], 80)

	rendered := strings.Join(lines, "\n")
	if !strings.Contains(rendered, "Start Plan") {
		t.Fatalf("expected start plan section, got %q", rendered)
	}
	if !strings.Contains(rendered, "Blocked") {
		t.Fatalf("expected blocked readiness, got %q", rendered)
	}
	if !strings.Contains(rendered, "Problem") {
		t.Fatalf("expected problem section, got %q", rendered)
	}
	if !strings.Contains(rendered, "missing-api") {
		t.Fatalf("expected missing profile name, got %q", rendered)
	}
	if !strings.Contains(rendered, "remove or replace missing member") {
		t.Fatalf("expected missing-profile fix hint, got %q", rendered)
	}
}

func TestRenderProfileDetailLinesShowsWarnings(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
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
	}, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	lines := model.renderProfileDetailLines(service.ProfileViews()[0], 80)
	rendered := strings.Join(lines, "\n")

	if !strings.Contains(rendered, "Ready with warnings") {
		t.Fatalf("expected warning readiness, got %q", rendered)
	}
	if !strings.Contains(rendered, "Warning") {
		t.Fatalf("expected warning rows, got %q", rendered)
	}
	if !strings.Contains(rendered, "still marked as draft") {
		t.Fatalf("expected draft warning text, got %q", rendered)
	}
	if !strings.Contains(rendered, "finish the draft fields") {
		t.Fatalf("expected actionable warning fix, got %q", rendered)
	}
}

func TestRenderStackDetailLinesShowsWarningMembers(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
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
	}, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	lines := model.renderStackDetailLines(service.StackViews()[0], 80)
	rendered := strings.Join(lines, "\n")

	if !strings.Contains(rendered, "Warnings") {
		t.Fatalf("expected warning count row, got %q", rendered)
	}
	if !strings.Contains(rendered, "still marked as draft") {
		t.Fatalf("expected member warning text, got %q", rendered)
	}
	if !strings.Contains(rendered, "finish the draft fields") {
		t.Fatalf("expected warning fix hint, got %q", rendered)
	}
}

func TestRenderProfileDetailLinesShowsRecentFailureSignals(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	lines := model.renderProfileDetailLines(app.ProfileView{
		Profile: service.ProfileViews()[0].Profile,
		State: domain.RuntimeState{
			ProfileName:  "prod-db",
			Status:       domain.TunnelStatusRestarting,
			LastError:    "dial tcp timeout",
			LastExitCode: 255,
			RecentLogs: []domain.LogEntry{
				{Timestamp: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC), Source: domain.LogSourceStderr, Message: "dial tcp timeout"},
				{Timestamp: time.Date(2026, 3, 30, 10, 0, 1, 0, time.UTC), Source: domain.LogSourceSystem, Message: "restarting in 4s"},
			},
		},
	}, 100)

	rendered := strings.Join(lines, "\n")
	for _, snippet := range []string{"Recent Failure", "dial tcp timeout", "restarting in 4s"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in recent-failure section, got %q", snippet, rendered)
		}
	}
}

func TestRenderProfileDetailLinesShowsRecentRuntimeHistory(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	model := Model{service: service}
	lines := model.renderProfileDetailLines(app.ProfileView{
		Profile: service.ProfileViews()[0].Profile,
		State: domain.RuntimeState{
			ProfileName: "prod-db",
			Status:      domain.TunnelStatusRestarting,
			RecentLogs: []domain.LogEntry{
				{Timestamp: base.Add(-3 * time.Second), Source: domain.LogSourceSystem, Message: "starting command: ssh -L 15432:db.internal:5432 bastion-prod"},
				{Timestamp: base.Add(-2 * time.Second), Source: domain.LogSourceSystem, Message: "process started with pid 1234"},
				{Timestamp: base.Add(-1 * time.Second), Source: domain.LogSourceSystem, Message: "process exited with code 255"},
				{Timestamp: base, Source: domain.LogSourceSystem, Message: "restarting in 4s"},
			},
		},
	}, 100)

	rendered := strings.Join(lines, "\n")
	for _, snippet := range []string{"Recent Runtime", "process started with pid 1234", "process exited with code 255", "restarting in 4s"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in recent runtime section, got %q", snippet, rendered)
		}
	}
}

func TestRenderProfileDetailLinesShowsDraftGuideForImportedDraft(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:        "bastion-prod",
				Description: "Imported from ~/.ssh/config.",
				Type:        domain.TunnelTypeSSHLocal,
				LocalPort:   15432,
				Labels:      []string{"draft", "imported", "ssh-config"},
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "127.0.0.1",
					RemotePort: 80,
				},
			},
		},
	}, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	rendered := strings.Join(model.renderProfileDetailLines(service.ProfileViews()[0], 100), "\n")
	for _, snippet := range []string{"Draft Guide", "Remote Host", "~/.ssh/config"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in draft guide, got %q", snippet, rendered)
		}
	}
}

func TestRenderStackDetailLinesAggregateFailureReasons(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
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
				Name:      "worker-debug",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15433,
				SSH: &domain.SSHLocal{
					Host:       "bastion-dev",
					RemoteHost: "worker.internal",
					RemotePort: 9000,
				},
			},
		},
		Stacks: []domain.Stack{
			{
				Name:     "backend-dev",
				Profiles: []string{"api-debug", "worker-debug"},
			},
		},
	}

	service, err := app.NewService(cfg, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	lines := model.renderStackDetailLines(app.StackView{
		Stack: cfg.Stacks[0],
		Members: []app.ProfileView{
			{
				Profile: cfg.Profiles[0],
				State: domain.RuntimeState{
					ProfileName: "api-debug",
					Status:      domain.TunnelStatusFailed,
					LastError:   "dial tcp timeout",
				},
			},
			{
				Profile: cfg.Profiles[1],
				State: domain.RuntimeState{
					ProfileName: "worker-debug",
					Status:      domain.TunnelStatusFailed,
					ExitReason:  "dial tcp timeout",
					RecentLogs: []domain.LogEntry{
						{Timestamp: time.Date(2026, 3, 30, 10, 0, 2, 0, time.UTC), Source: domain.LogSourceSystem, Message: "restarting in 3s"},
					},
				},
			},
		},
	}, 120)

	rendered := strings.Join(lines, "\n")
	for _, snippet := range []string{"Recent Failures", "dial tcp timeout", "2 members", "restarting in 3s"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in stack failure aggregation, got %q", snippet, rendered)
		}
	}
}

func TestRenderStackDetailLinesShowsRecentRuntimeHistory(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
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
				Name:      "worker-debug",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15433,
				SSH: &domain.SSHLocal{
					Host:       "bastion-dev",
					RemoteHost: "worker.internal",
					RemotePort: 9000,
				},
			},
		},
		Stacks: []domain.Stack{
			{
				Name:     "backend-dev",
				Profiles: []string{"api-debug", "worker-debug"},
			},
		},
	}

	service, err := app.NewService(cfg, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	model := Model{service: service}
	lines := model.renderStackDetailLines(app.StackView{
		Stack: cfg.Stacks[0],
		Members: []app.ProfileView{
			{
				Profile: cfg.Profiles[0],
				State: domain.RuntimeState{
					ProfileName: "api-debug",
					RecentLogs: []domain.LogEntry{
						{Timestamp: base.Add(-1 * time.Second), Source: domain.LogSourceSystem, Message: "process exited with code 255"},
					},
				},
			},
			{
				Profile: cfg.Profiles[1],
				State: domain.RuntimeState{
					ProfileName: "worker-debug",
					RecentLogs: []domain.LogEntry{
						{Timestamp: base, Source: domain.LogSourceSystem, Message: "retrying after launch failure in 2s"},
					},
				},
			},
		},
	}, 120)

	rendered := strings.Join(lines, "\n")
	for _, snippet := range []string{"Recent Runtime", "api-debug", "worker-debug", "process exited with code 255", "retrying after launch failure in 2s"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in stack recent runtime section, got %q", snippet, rendered)
		}
	}
}

func TestRenderStackDetailLinesShowsDraftGuide(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
		Stacks: []domain.Stack{
			{
				Name:   "draft-stack",
				Labels: []string{"draft"},
			},
		},
	}

	service, err := app.NewService(cfg, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	rendered := strings.Join(model.renderStackDetailLines(service.StackViews()[0], 120), "\n")
	for _, snippet := range []string{"Draft Guide", "Member 1", "comma/newline-separated"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in stack draft guide, got %q", snippet, rendered)
		}
	}
}

func TestRenderProfileDetailLinesUsesChineseWhenConfigured(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.Config{
		Version:  domain.CurrentConfigVersion,
		Language: domain.LanguageSimplifiedChinese,
		Profiles: []domain.Profile{
			{
				Name:      "api-debug",
				Type:      domain.TunnelTypeKubernetesPortForward,
				LocalPort: 8080,
				Restart: domain.RestartPolicy{
					Enabled: true,
				},
				Kubernetes: &domain.Kubernetes{
					Context:      "dev-cluster",
					Namespace:    "backend",
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
		},
	}, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service: service,
		now:     time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC),
	}

	rendered := strings.Join(model.renderProfileDetailLines(service.ProfileViews()[0], 80), "\n")
	for _, snippet := range []string{"概览", "运行态", "就绪度", "命令"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in Chinese detail view, got %q", snippet, rendered)
		}
	}
}

func TestRenderQuickActionRowsUsesTwoColumnsWhenWide(t *testing.T) {
	t.Parallel()

	rows := renderQuickActionRows(48, []quickAction{
		{key: "i", label: "import drafts"},
		{key: "s", label: "sample config"},
		{key: "e", label: "edit config"},
		{key: "g", label: "reload config"},
	})

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if got := lipgloss.Width(row); got != 48 {
			t.Fatalf("expected row width 48, got %d (%q)", got, row)
		}
	}

	rendered := strings.Join(rows, "\n")
	for _, snippet := range []string{"import drafts", "sample config", "edit config", "reload config"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in quick action rows, got %q", snippet, rendered)
		}
	}
}

func TestRenderQuickActionRowsFallsBackToSingleColumnWhenNarrow(t *testing.T) {
	t.Parallel()

	rows := renderQuickActionRows(24, []quickAction{
		{key: "A", label: "draft stack"},
		{key: "e", label: "edit config"},
		{key: "g", label: "reload config"},
		{key: "Tab", label: "focus profiles"},
	})

	if len(rows) != 4 {
		t.Fatalf("expected 4 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if got := lipgloss.Width(row); got != 24 {
			t.Fatalf("expected row width 24, got %d (%q)", got, row)
		}
	}

	rendered := strings.Join(rows, "\n")
	for _, snippet := range []string{"draft stack", "edit config", "reload config", "focus profiles"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in quick action rows, got %q", snippet, rendered)
		}
	}
}

func TestRenderEmptyProfilesLinesIncludesAllActions(t *testing.T) {
	t.Parallel()

	model := Model{}
	lines := model.renderEmptyProfilesLines(48)
	rendered := strings.Join(lines, "\n")

	for _, snippet := range []string{"import drafts", "sample config", "profile preset", "guided editor", "raw YAML", "reload config"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in empty profiles lines, got %q", snippet, rendered)
		}
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

func TestRenderProfileRowMarksSelection(t *testing.T) {
	t.Parallel()

	model := Model{}
	view := app.ProfileView{
		Profile: domain.Profile{
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
	}

	focused := model.renderProfileRow(view, true, true, 80)
	if !strings.Contains(focused, "> ") {
		t.Fatalf("expected focused selected marker, got %q", focused)
	}

	outlined := model.renderProfileRow(view, true, false, 80)
	if !strings.Contains(outlined, "| ") {
		t.Fatalf("expected unfocused selected marker, got %q", outlined)
	}

	plain := model.renderProfileRow(view, false, false, 80)
	if strings.Contains(plain, "> ") || strings.Contains(plain, "| ") {
		t.Fatalf("expected unselected row to omit selection marker, got %q", plain)
	}
}

func TestProfileActionLinesIncludeRestart(t *testing.T) {
	t.Parallel()

	model := Model{}
	lines := model.profileActionLines(app.ProfileView{}, 40)
	rendered := strings.Join(lines, "\n")

	if !strings.Contains(rendered, "r") || !strings.Contains(rendered, "restart tunnel") {
		t.Fatalf("expected restart action in profile actions, got %q", rendered)
	}
	if !strings.Contains(rendered, "g") || !strings.Contains(rendered, "reload config from disk") {
		t.Fatalf("expected reload action in profile actions, got %q", rendered)
	}
}

func TestRenderInspectorTabsShowsKeyHints(t *testing.T) {
	t.Parallel()

	model := Model{inspectorTab: inspectorTabLogs}
	got := model.renderInspectorTabs(40)

	if !strings.Contains(got, "h Details") {
		t.Fatalf("expected details tab hint, got %q", got)
	}
	if !strings.Contains(got, "l Logs") {
		t.Fatalf("expected logs tab hint, got %q", got)
	}
	if !strings.Contains(got, "/ filter") {
		t.Fatalf("expected log filter tab hint, got %q", got)
	}
}

func TestTransientNoticeAutoHidesAndMouseLayoutReflows(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	base := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	model := Model{
		service: service,
		width:   140,
		height:  32,
		now:     base,
	}
	model.setNotice("Switched language to English.")

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	before := model.mouseLayout(profiles, stacks)
	if model.renderStatusLine(model.contentWidth()) == "" {
		t.Fatal("expected notice status line to be visible before ttl")
	}

	model.now = base.Add(noticeTTL + time.Second)
	if model.renderStatusLine(model.contentWidth()) != "" {
		t.Fatal("expected notice status line to hide after ttl")
	}
	if model.hasStatusLine() {
		t.Fatal("expected status line accounting to hide after ttl")
	}

	after := model.mouseLayout(profiles, stacks)
	if before.profiles.panel.y <= after.profiles.panel.y {
		t.Fatalf("expected profiles panel to move up after notice hides, before=%d after=%d", before.profiles.panel.y, after.profiles.panel.y)
	}

	x := panelContentX(after.profiles.panel)
	y := panelBodyStartY(after.profiles.panel)
	if idx, ok := after.profiles.rowIndexAt(x, y); !ok || idx != 0 {
		t.Fatalf("expected first profile row to remain clickable after reflow, got idx=%d ok=%v", idx, ok)
	}
}

func TestNormalizeLogMessageCollapsesMultilineWhitespace(t *testing.T) {
	t.Parallel()

	got := normalizeLogMessage(" first line \n\n second\tline \r\n third line ")
	want := "first line | second line | third line"
	if got != want {
		t.Fatalf("normalizeLogMessage() = %q, want %q", got, want)
	}
}

func TestRenderLogLineShowsProfileBadgeAndNormalizedMessage(t *testing.T) {
	t.Parallel()

	got := renderLogLine(
		time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC),
		"api-debug",
		domain.LogSourceStderr,
		"boom\nsecond line",
		"",
		120,
	)

	for _, snippet := range []string{"11:00:00", "ERR", "api-debug", "boom | second line"} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected %q in rendered log line, got %q", snippet, got)
		}
	}
}

func TestRenderProfileLogLinesIncludeSummaryAndFilterState(t *testing.T) {
	t.Parallel()

	view := app.ProfileView{
		State: domain.RuntimeState{
			RecentLogs: []domain.LogEntry{
				{
					Timestamp: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
					Source:    domain.LogSourceStdout,
					Message:   "server ready",
				},
				{
					Timestamp: time.Date(2026, 3, 30, 10, 0, 1, 0, time.UTC),
					Source:    domain.LogSourceStderr,
					Message:   "dial tcp timeout",
				},
			},
		},
	}

	model := Model{logFilterQuery: "timeout"}
	lines := model.renderProfileLogLines(view, 120)
	rendered := strings.Join(lines, "\n")

	for _, snippet := range []string{"Showing 1/2 logs", "Sources", "Filter:"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in profile log summary, got %q", snippet, rendered)
		}
	}
	if !strings.Contains(rendered, renderLogSourceBadge(domain.LogSourceStderr, "")) {
		t.Fatalf("expected stderr badge in profile log summary, got %q", rendered)
	}
	if !strings.Contains(rendered, filterMatchStyle.Render("timeout")) {
		t.Fatalf("expected highlighted filter query in summary, got %q", rendered)
	}
}

func TestRenderProfileLogContentAppliesSourceFilterAndWrap(t *testing.T) {
	t.Parallel()

	view := app.ProfileView{
		State: domain.RuntimeState{
			RecentLogs: []domain.LogEntry{
				{
					Timestamp: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
					Source:    domain.LogSourceStdout,
					Message:   "stdout ready",
				},
				{
					Timestamp: time.Date(2026, 3, 30, 10, 0, 1, 0, time.UTC),
					Source:    domain.LogSourceStderr,
					Message:   "dial tcp timeout while waiting for the upstream server to finish booting",
				},
			},
		},
	}

	model := Model{
		logSourceFilter: domain.LogSourceStderr,
		logWrap:         true,
	}
	content := model.renderProfileLogContent(view, 44)
	rendered := strings.Join(content.lines, "\n")

	if strings.Contains(rendered, "stdout ready") {
		t.Fatalf("expected stdout log to be filtered out, got %q", rendered)
	}
	if !strings.Contains(rendered, "dial tcp timeout") {
		t.Fatalf("expected stderr log to remain visible, got %q", rendered)
	}
	if len(content.logHitStarts) != 1 {
		t.Fatalf("expected source-filtered content to expose one match hit, got %#v", content.logHitStarts)
	}
	if len(content.lines) <= 4 {
		t.Fatalf("expected wrapped content to use multiple lines, got %#v", content.lines)
	}
}

func TestRenderStackLogLinesIncludeProfileCoverageSummary(t *testing.T) {
	t.Parallel()

	view := app.StackView{
		Members: []app.ProfileView{
			{
				Profile: domain.Profile{Name: "api-debug"},
				State: domain.RuntimeState{
					RecentLogs: []domain.LogEntry{
						{
							Timestamp: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
							Source:    domain.LogSourceStdout,
							Message:   "api ready",
						},
					},
				},
			},
			{
				Profile: domain.Profile{Name: "worker-debug"},
				State: domain.RuntimeState{
					RecentLogs: []domain.LogEntry{
						{
							Timestamp: time.Date(2026, 3, 30, 10, 0, 1, 0, time.UTC),
							Source:    domain.LogSourceSystem,
							Message:   "worker started",
						},
					},
				},
			},
		},
	}

	model := Model{}
	lines := model.renderStackLogLines(view, 120)
	rendered := strings.Join(lines, "\n")

	for _, snippet := range []string{"Showing 2 logs from 2 profiles", "Sources"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in stack log summary, got %q", snippet, rendered)
		}
	}
	for _, badge := range []string{
		renderLogSourceBadge(domain.LogSourceSystem, ""),
		renderLogSourceBadge(domain.LogSourceStdout, ""),
	} {
		if !strings.Contains(rendered, badge) {
			t.Fatalf("expected %q in stack log summary, got %q", badge, rendered)
		}
	}
}

func TestRenderHeaderFilterSegmentUsesLogContextInLogsTab(t *testing.T) {
	t.Parallel()

	model := Model{inspectorTab: inspectorTabLogs}
	got := model.renderHeaderFilterSegment(28)

	if !strings.Contains(got, "Logs") {
		t.Fatalf("expected logs label, got %q", got)
	}
	if !strings.Contains(got, "message, source, profile") {
		t.Fatalf("expected log filter placeholder, got %q", got)
	}
}

func TestTruncateTextUsesDisplayWidthForWideCharacters(t *testing.T) {
	t.Parallel()

	got := truncateText("配置组合日志关键字", 8)

	if width := ansi.StringWidth(got); width > 8 {
		t.Fatalf("expected truncated width <= 8, got %d (%q)", width, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected wide-character truncation to add ellipsis, got %q", got)
	}
}

func TestChineseViewDoesNotOverflowViewportWidth(t *testing.T) {
	t.Parallel()

	cfg := domain.DefaultConfig()
	cfg.Language = domain.LanguageSimplifiedChinese

	service, err := app.NewService(cfg, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: "/Users/urzeye/workspace/lazytunnel/config.yaml",
		width:      72,
		height:     20,
		importMode: true,
	}

	for idx, line := range strings.Split(model.View(), "\n") {
		if width := ansi.StringWidth(line); width > model.width {
			t.Fatalf("expected rendered line %d to fit width %d, got %d (%q)", idx, model.width, width, line)
		}
	}
}

func TestRenderStatusBadgeUsesReadableWords(t *testing.T) {
	t.Parallel()

	if got := renderStatusBadge(domain.LanguageEnglish, domain.TunnelStatusStopped); !strings.Contains(got, "STOP") {
		t.Fatalf("expected STOP badge, got %q", got)
	}
	if got := renderStatusBadge(domain.LanguageEnglish, domain.TunnelStatusRestarting); !strings.Contains(got, "RETRY") {
		t.Fatalf("expected RETRY badge, got %q", got)
	}
	if got := renderStatusBadge(domain.LanguageEnglish, domain.TunnelStatusStarting); !strings.Contains(got, "START") {
		t.Fatalf("expected START badge, got %q", got)
	}
}

func TestRenderStackStatusBadgeUsesReadableWords(t *testing.T) {
	t.Parallel()

	if got := renderStackStatusBadge(domain.LanguageEnglish, app.StackStatusStopped); !strings.Contains(got, "STOP") {
		t.Fatalf("expected STOP stack badge, got %q", got)
	}
	if got := renderStackStatusBadge(domain.LanguageEnglish, app.StackStatusPartial); !strings.Contains(got, "PART") {
		t.Fatalf("expected PART stack badge, got %q", got)
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

	got := formatLastExit(domain.LanguageEnglish, state, now)
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

func TestFilterStackViewsMatchesStackNameAndDeclaredProfiles(t *testing.T) {
	t.Parallel()

	views := []app.StackView{
		{
			Stack: domain.Stack{
				Name:     "backend-dev",
				Profiles: []string{"prod-db", "missing-worker"},
			},
			Members: []app.ProfileView{
				{Profile: domain.Profile{Name: "prod-db"}},
			},
		},
	}

	if got := filterStackViews(views, "backend-dev"); len(got) != 1 || got[0].Stack.Name != "backend-dev" {
		t.Fatalf("expected stack-name filter to match backend-dev, got %#v", got)
	}

	if got := filterStackViews(views, "missing-worker"); len(got) != 1 || got[0].Stack.Name != "backend-dev" {
		t.Fatalf("expected declared-member filter to match backend-dev, got %#v", got)
	}
}

func TestFilterLogEntriesMatchesMessageAndSource(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC)
	entries := []domain.LogEntry{
		{Timestamp: base, Source: domain.LogSourceSystem, Message: "process started"},
		{Timestamp: base.Add(time.Second), Source: domain.LogSourceStderr, Message: "dial tcp timeout"},
	}

	if got := filterLogEntries(entries, "stderr"); len(got) != 1 || got[0].Message != "dial tcp timeout" {
		t.Fatalf("expected stderr filter to match error log, got %#v", got)
	}

	if got := filterLogEntries(entries, "started"); len(got) != 1 || got[0].Message != "process started" {
		t.Fatalf("expected message filter to match system log, got %#v", got)
	}
}

func TestFilterStackActivityMatchesProfileName(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 3, 28, 11, 0, 0, 0, time.UTC)
	entries := []stackActivityEntry{
		{
			ProfileName: "prod-db",
			Log: domain.LogEntry{
				Timestamp: base,
				Source:    domain.LogSourceStdout,
				Message:   "ready",
			},
		},
		{
			ProfileName: "api-debug",
			Log: domain.LogEntry{
				Timestamp: base.Add(time.Second),
				Source:    domain.LogSourceSystem,
				Message:   "process started",
			},
		},
	}

	if got := filterStackActivity(entries, "api-debug"); len(got) != 1 || got[0].ProfileName != "api-debug" {
		t.Fatalf("expected profile-name filter to match api-debug, got %#v", got)
	}
}

func TestRenderProfileRowHighlightsMatchedFilter(t *testing.T) {
	t.Parallel()

	model := Model{filterQuery: "prod"}
	row := model.renderProfileRow(app.ProfileView{
		Profile: domain.Profile{
			Name:      "prod-db",
			Type:      domain.TunnelTypeSSHLocal,
			LocalPort: 5432,
			SSH: &domain.SSHLocal{
				Host:       "bastion-prod",
				RemoteHost: "db.internal",
				RemotePort: 5432,
			},
		},
	}, false, true, 80)

	if !strings.Contains(row, filterMatchStyle.Render("prod")) {
		t.Fatalf("expected highlighted profile match, got %q", row)
	}
}

func TestRenderStackRowHighlightsMatchedFilter(t *testing.T) {
	t.Parallel()

	model := Model{filterQuery: "api"}
	row := model.renderStackRow(app.StackView{
		Stack: domain.Stack{
			Name:     "backend-dev",
			Profiles: []string{"prod-db", "api-debug"},
		},
		Members: []app.ProfileView{
			{Profile: domain.Profile{Name: "prod-db"}},
			{Profile: domain.Profile{Name: "api-debug"}},
		},
	}, false, true, 80)

	if !strings.Contains(row, filterMatchStyle.Render("api")) {
		t.Fatalf("expected highlighted stack match, got %q", row)
	}
}

func TestRenderLogLineHighlightsMessageAndBadges(t *testing.T) {
	t.Parallel()

	line := renderLogLine(
		time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
		"api-debug",
		domain.LogSourceStderr,
		"dial tcp timeout",
		"timeout",
		120,
	)
	if !strings.Contains(line, filterMatchStyle.Render("timeout")) {
		t.Fatalf("expected highlighted log message match, got %q", line)
	}

	badgeLine := renderLogLine(
		time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
		"api-debug",
		domain.LogSourceStderr,
		"dial tcp timeout",
		"stderr",
		120,
	)
	if !strings.Contains(badgeLine, logMatchBadgeStyle.Render("ERR")) {
		t.Fatalf("expected highlighted source badge, got %q", badgeLine)
	}

	profileLine := renderLogLine(
		time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
		"api-debug",
		domain.LogSourceStdout,
		"ready",
		"api",
		120,
	)
	if !strings.Contains(profileLine, logMatchBadgeStyle.Render("api-debug")) {
		t.Fatalf("expected highlighted profile badge, got %q", profileLine)
	}
}

func TestHandleFilterKeyUpdatesLogFilterIndependently(t *testing.T) {
	t.Parallel()

	model := Model{
		filterQuery:     "prod",
		logFilterQuery:  "",
		filterMode:      true,
		filterScope:     filterScopeLogs,
		inspectorTab:    inspectorTabLogs,
		selectedProfile: 1,
		selectedStack:   1,
	}

	next, handled := model.handleFilterKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !handled {
		t.Fatal("expected log filter key to be handled")
	}
	if next.logFilterQuery != "e" {
		t.Fatalf("expected log filter query to update, got %q", next.logFilterQuery)
	}
	if next.filterQuery != "prod" {
		t.Fatalf("expected list filter query to stay unchanged, got %q", next.filterQuery)
	}
	if next.selectedProfile != 1 || next.selectedStack != 1 {
		t.Fatalf("expected selection to stay unchanged, got profile=%d stack=%d", next.selectedProfile, next.selectedStack)
	}
}

func TestHandleInspectorKeyHomeAndEndNavigateLogs(t *testing.T) {
	t.Parallel()

	logs := make([]domain.LogEntry, 0, 40)
	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		logs = append(logs, domain.LogEntry{
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Source:    domain.LogSourceStdout,
			Message:   fmt.Sprintf("log line %02d", i),
		})
	}

	runtime := newStubRuntimeController()
	runtime.states["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
		RecentLogs:  logs,
	}

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}

	service, err := app.NewService(cfg, runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:         service,
		width:           140,
		height:          30,
		inspectorTab:    inspectorTabLogs,
		inspectorScroll: 5,
	}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")

	next, handled := model.handleInspectorKey(tea.KeyMsg{Type: tea.KeyHome}, profiles, stacks)
	if !handled {
		t.Fatal("expected home key to be handled")
	}
	if next.inspectorScroll != 0 {
		t.Fatalf("expected home to jump to latest logs, got scroll=%d", next.inspectorScroll)
	}

	next, handled = next.handleInspectorKey(tea.KeyMsg{Type: tea.KeyEnd}, profiles, stacks)
	if !handled {
		t.Fatal("expected end key to be handled")
	}
	if next.inspectorScroll <= 0 {
		t.Fatalf("expected end to jump away from latest logs, got scroll=%d", next.inspectorScroll)
	}
}

func TestHandleInspectorKeyManagesLogFollowPauseAndSourceFilter(t *testing.T) {
	t.Parallel()

	logs := []domain.LogEntry{
		{Timestamp: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC), Source: domain.LogSourceStdout, Message: "server ready"},
		{Timestamp: time.Date(2026, 3, 30, 10, 0, 1, 0, time.UTC), Source: domain.LogSourceStderr, Message: "dial tcp timeout"},
		{Timestamp: time.Date(2026, 3, 30, 10, 0, 2, 0, time.UTC), Source: domain.LogSourceStdout, Message: "health ok"},
		{Timestamp: time.Date(2026, 3, 30, 10, 0, 3, 0, time.UTC), Source: domain.LogSourceStderr, Message: "timeout waiting for upstream"},
	}

	runtime := newStubRuntimeController()
	runtime.states["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
		RecentLogs:  logs,
	}

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}

	service, err := app.NewService(cfg, runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:        service,
		width:          140,
		height:         30,
		inspectorTab:   inspectorTabLogs,
		logFilterQuery: "timeout",
	}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")

	next, handled := model.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected n to navigate log matches")
	}
	if next.inspectorScroll == 0 {
		t.Fatalf("expected next-match navigation to move below the summary, got %d", next.inspectorScroll)
	}
	if !next.logFollowPaused {
		t.Fatal("expected next-match navigation to pause follow mode")
	}

	next, handled = next.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected f to toggle follow mode")
	}
	if next.logFollowPaused || next.inspectorScroll != 0 {
		t.Fatalf("expected follow resume to reset scroll, got paused=%v scroll=%d", next.logFollowPaused, next.inspectorScroll)
	}

	next, handled = next.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected t to cycle log source filter")
	}
	if next.logSourceFilter != domain.LogSourceSystem {
		t.Fatalf("expected first source cycle to choose system logs, got %q", next.logSourceFilter)
	}
}

func TestHandleInspectorKeyUsesYToCopyExecCommand(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var copied string
	model := Model{
		service:         service,
		clipboardWriter: func(text string) error { copied = text; return nil },
	}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")

	next, handled := model.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected y to copy the current exec command")
	}
	if !strings.Contains(copied, "ssh ") || !strings.Contains(copied, "-L") {
		t.Fatalf("expected copied ssh local command, got %q", copied)
	}
	if !strings.Contains(next.lastNotice, "Copied exec command") {
		t.Fatalf("expected copy-command notice, got %q", next.lastNotice)
	}
}

func TestHandleInspectorKeyLogActionsCopyExportAndClear(t *testing.T) {
	t.Parallel()

	runtime := newStubRuntimeController()
	runtime.states["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
		RecentLogs: []domain.LogEntry{
			{Timestamp: time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC), Source: domain.LogSourceStdout, Message: "server ready"},
			{Timestamp: time.Date(2026, 3, 30, 10, 0, 1, 0, time.UTC), Source: domain.LogSourceStderr, Message: "dial tcp timeout"},
		},
	}

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}

	service, err := app.NewService(cfg, runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	var copied string
	var exportedBase string
	var exportedContent string
	model := Model{
		service:         service,
		inspectorTab:    inspectorTabLogs,
		clipboardWriter: func(text string) error { copied = text; return nil },
		textExporter: func(baseName, content string) (string, error) {
			exportedBase = baseName
			exportedContent = content
			return "/tmp/prod-db.log", nil
		},
	}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")

	next, handled := model.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected y to copy visible logs")
	}
	if !strings.Contains(copied, "server ready") || !strings.Contains(copied, "dial tcp timeout") {
		t.Fatalf("expected copied log snapshot, got %q", copied)
	}

	next, handled = next.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected o to export visible logs")
	}
	if exportedBase != "prod-db" {
		t.Fatalf("expected exported base name prod-db, got %q", exportedBase)
	}
	if !strings.Contains(exportedContent, "server ready") {
		t.Fatalf("expected exported logs to include content, got %q", exportedContent)
	}

	next, handled = next.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected x to clear visible logs")
	}
	if got := len(runtime.states["prod-db"].RecentLogs); got != 0 {
		t.Fatalf("expected logs to be cleared, got %d", got)
	}
	if !strings.Contains(next.lastNotice, "Cleared logs for profile prod-db.") {
		t.Fatalf("expected clear-logs notice, got %q", next.lastNotice)
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

func TestNextProfileDraftNameAddsSuffixWhenNeeded(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{Name: "draft-ssh"},
			{Name: "draft-ssh-2"},
		},
	}

	if got := nextProfileDraftName(cfg, "draft-ssh"); got != "draft-ssh-3" {
		t.Fatalf("nextProfileDraftName() = %q, want %q", got, "draft-ssh-3")
	}
}

func TestNextAvailableLocalPortSkipsUsedPorts(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{LocalPort: 15432},
			{LocalPort: 15433},
		},
	}

	if got := nextAvailableLocalPort(cfg, 15432); got != 15434 {
		t.Fatalf("nextAvailableLocalPort() = %d, want %d", got, 15434)
	}
}

func TestNextStackDraftNameAddsSuffixWhenNeeded(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Stacks: []domain.Stack{
			{Name: "draft-stack"},
			{Name: "draft-stack-2"},
		},
	}

	if got := nextStackDraftName(cfg, "draft-stack"); got != "draft-stack-3" {
		t.Fatalf("nextStackDraftName() = %q, want %q", got, "draft-stack-3")
	}
}

func TestNextCopyNameAddsSuffixWhenNeeded(t *testing.T) {
	t.Parallel()

	if got := nextCopyName([]string{"prod-db"}, "prod-db"); got != "prod-db-copy" {
		t.Fatalf("nextCopyName() = %q, want %q", got, "prod-db-copy")
	}

	existing := []string{"prod-db", "prod-db-copy", "prod-db-copy-2"}
	if got := nextCopyName(existing, "prod-db"); got != "prod-db-copy-3" {
		t.Fatalf("nextCopyName() = %q, want %q", got, "prod-db-copy-3")
	}
}

func TestAppendUniqueLabelAvoidsDuplicates(t *testing.T) {
	t.Parallel()

	labels := []string{"prod"}
	got := appendUniqueLabel(labels, "draft")
	if want := "prod,draft"; strings.Join(got, ",") != want {
		t.Fatalf("appendUniqueLabel() = %q, want %q", strings.Join(got, ","), want)
	}
	if want := "prod"; strings.Join(labels, ",") != want {
		t.Fatalf("expected original labels unchanged, got %q", strings.Join(labels, ","))
	}

	got = appendUniqueLabel([]string{"prod", "draft"}, "draft")
	if want := "prod,draft"; strings.Join(got, ",") != want {
		t.Fatalf("appendUniqueLabel() with existing draft = %q, want %q", strings.Join(got, ","), want)
	}
}

func TestCreateStarterProfileDraftPersistsAndSelects(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: configPath,
	}

	model = model.createStarterProfileDraft()

	if model.lastError != "" {
		t.Fatalf("expected no error, got %q", model.lastError)
	}
	if !strings.Contains(model.lastNotice, "Created starter profile") {
		t.Fatalf("expected creation notice, got %q", model.lastNotice)
	}
	if model.editor == nil || model.editor.kind != formEditorProfile {
		t.Fatalf("expected profile editor to open, got %#v", model.editor)
	}

	views := model.service.ProfileViews()
	if len(views) != 1 {
		t.Fatalf("expected 1 profile view, got %d", len(views))
	}
	if got := views[model.selectedProfile].Profile.Name; got != "draft-ssh" {
		t.Fatalf("expected selected profile draft-ssh, got %q", got)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected persisted profile, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles[0].SSH == nil || cfg.Profiles[0].SSH.Host != "example-bastion" {
		t.Fatalf("unexpected persisted profile: %#v", cfg.Profiles[0])
	}
}

func TestCreateStarterStackDraftPersistsAndSelects(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:         service,
		configPath:      configPath,
		selectedProfile: 1,
	}

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	model = model.createStarterStackDraft(profiles, stacks)

	if model.lastError != "" {
		t.Fatalf("expected no error, got %q", model.lastError)
	}
	if !strings.Contains(model.lastNotice, "Created starter stack") {
		t.Fatalf("expected creation notice, got %q", model.lastNotice)
	}
	if model.editor == nil || model.editor.kind != formEditorStack {
		t.Fatalf("expected stack editor to open, got %#v", model.editor)
	}

	stackViews := model.service.StackViews()
	if len(stackViews) != 2 {
		t.Fatalf("expected 2 stack views, got %d", len(stackViews))
	}
	if got := stackViews[model.selectedStack].Stack.Name; got != "draft-stack" {
		t.Fatalf("expected selected stack draft-stack, got %q", got)
	}
	if got := strings.Join(stackViews[model.selectedStack].Stack.Profiles, ","); got != "api-debug" {
		t.Fatalf("expected stack members api-debug, got %q", got)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Stacks) != 2 {
		t.Fatalf("expected persisted draft stack, got %d stacks", len(cfg.Stacks))
	}
}

func TestCloneSelectedProfilePersistsAndSelects(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := storage.SampleConfig()
	occupiedPort := cfg.Profiles[0]
	occupiedPort.Name = "prod-db-shadow"
	occupiedPort.LocalPort = 5433
	cfg.Profiles = append(cfg.Profiles, occupiedPort)

	service, err := app.NewService(cfg, newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:         service,
		configPath:      configPath,
		selectedProfile: 0,
		selectedStack:   1,
		filterQuery:     "prod",
		filterMode:      true,
	}

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	model = model.cloneSelection(profiles, stacks)

	if model.lastError != "" {
		t.Fatalf("expected no error, got %q", model.lastError)
	}
	if !strings.Contains(model.lastNotice, "Cloned profile prod-db to prod-db-copy.") {
		t.Fatalf("expected clone notice, got %q", model.lastNotice)
	}
	if model.focus != focusProfiles {
		t.Fatalf("expected profiles focus, got %v", model.focus)
	}
	if model.filterQuery != "" || model.filterMode {
		t.Fatalf("expected filter to reset, got query=%q mode=%v", model.filterQuery, model.filterMode)
	}
	if model.selectedStack != 0 {
		t.Fatalf("expected stack selection reset, got %d", model.selectedStack)
	}

	views := model.service.ProfileViews()
	if len(views) != 4 {
		t.Fatalf("expected 4 profile views, got %d", len(views))
	}

	selected := views[model.selectedProfile].Profile
	if selected.Name != "prod-db-copy" {
		t.Fatalf("expected selected clone prod-db-copy, got %q", selected.Name)
	}
	if selected.LocalPort != 5434 {
		t.Fatalf("expected cloned profile to use port 5434, got %d", selected.LocalPort)
	}
	if !containsLabel(selected.Labels, "draft") {
		t.Fatalf("expected cloned profile to include draft label, got %#v", selected.Labels)
	}
	if containsLabel(views[0].Profile.Labels, "draft") {
		t.Fatalf("expected original profile labels unchanged, got %#v", views[0].Profile.Labels)
	}

	persisted, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	cloned, ok := findProfileByName(persisted.Profiles, "prod-db-copy")
	if !ok {
		t.Fatalf("expected persisted cloned profile, got %#v", persisted.Profiles)
	}
	if cloned.LocalPort != 5434 {
		t.Fatalf("expected persisted clone port 5434, got %d", cloned.LocalPort)
	}
	if !containsLabel(cloned.Labels, "draft") {
		t.Fatalf("expected persisted cloned profile to include draft label, got %#v", cloned.Labels)
	}
}

func TestCloneSelectedStackPersistsAndSelects(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:         service,
		configPath:      configPath,
		focus:           focusStacks,
		selectedProfile: 1,
		selectedStack:   0,
		filterQuery:     "backend",
		filterMode:      true,
	}

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	model = model.cloneSelection(profiles, stacks)

	if model.lastError != "" {
		t.Fatalf("expected no error, got %q", model.lastError)
	}
	if !strings.Contains(model.lastNotice, "Cloned stack backend-dev to backend-dev-copy.") {
		t.Fatalf("expected clone notice, got %q", model.lastNotice)
	}
	if model.focus != focusStacks {
		t.Fatalf("expected stacks focus, got %v", model.focus)
	}
	if model.filterQuery != "" || model.filterMode {
		t.Fatalf("expected filter to reset, got query=%q mode=%v", model.filterQuery, model.filterMode)
	}
	if model.selectedProfile != 0 {
		t.Fatalf("expected profile selection reset, got %d", model.selectedProfile)
	}

	stackViews := model.service.StackViews()
	if len(stackViews) != 2 {
		t.Fatalf("expected 2 stack views, got %d", len(stackViews))
	}

	selected := stackViews[model.selectedStack].Stack
	if selected.Name != "backend-dev-copy" {
		t.Fatalf("expected selected clone backend-dev-copy, got %q", selected.Name)
	}
	if got := strings.Join(selected.Profiles, ","); got != "prod-db,api-debug" {
		t.Fatalf("expected cloned stack members prod-db,api-debug, got %q", got)
	}
	if !containsLabel(selected.Labels, "draft") {
		t.Fatalf("expected cloned stack to include draft label, got %#v", selected.Labels)
	}
	if containsLabel(stackViews[0].Stack.Labels, "draft") {
		t.Fatalf("expected original stack labels unchanged, got %#v", stackViews[0].Stack.Labels)
	}

	persisted, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	cloned, ok := findStackByName(persisted.Stacks, "backend-dev-copy")
	if !ok {
		t.Fatalf("expected persisted cloned stack, got %#v", persisted.Stacks)
	}
	if got := strings.Join(cloned.Profiles, ","); got != "prod-db,api-debug" {
		t.Fatalf("expected persisted cloned stack members prod-db,api-debug, got %q", got)
	}
	if !containsLabel(cloned.Labels, "draft") {
		t.Fatalf("expected persisted cloned stack to include draft label, got %#v", cloned.Labels)
	}
}

func TestCreateStarterStackDraftNeedsVisibleProfile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:     service,
		configPath:  configPath,
		filterQuery: "missing",
	}

	model = model.createStarterStackDraft(nil, nil)

	if !strings.Contains(model.lastError, "No visible profile") {
		t.Fatalf("expected visible profile error, got %q", model.lastError)
	}
}

func TestCreateStarterStackDraftFromSelectedStackCopiesMembers(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:       service,
		configPath:    configPath,
		focus:         focusStacks,
		selectedStack: 0,
	}

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	model = model.createStarterStackDraft(profiles, stacks)

	if model.lastError != "" {
		t.Fatalf("expected no error, got %q", model.lastError)
	}

	stackViews := model.service.StackViews()
	if len(stackViews) != 2 {
		t.Fatalf("expected 2 stack views, got %d", len(stackViews))
	}
	if got := strings.Join(stackViews[model.selectedStack].Stack.Profiles, ","); got != "prod-db,api-debug" {
		t.Fatalf("expected copied members prod-db,api-debug, got %q", got)
	}
}

func TestReloadConfigFromDiskReplacesServiceConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: configPath,
	}

	model = model.reloadConfigFromDisk("Reloaded config from disk.")

	if model.lastError != "" {
		t.Fatalf("expected no error, got %q", model.lastError)
	}
	if got := len(model.service.ProfileViews()); got != 2 {
		t.Fatalf("expected 2 reloaded profiles, got %d", got)
	}
	if got := len(model.service.StackViews()); got != 1 {
		t.Fatalf("expected 1 reloaded stack, got %d", got)
	}
}

func TestToggleLanguagePersistsAndReloadsServiceConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: configPath,
	}

	model = model.toggleLanguage()

	if model.lastError != "" {
		t.Fatalf("expected no error, got %q", model.lastError)
	}
	if got := model.service.Config().Language; got != domain.LanguageSimplifiedChinese {
		t.Fatalf("expected service language zh-CN, got %q", got)
	}
	if !strings.Contains(model.lastNotice, "简体中文") {
		t.Fatalf("expected switch notice to mention Chinese, got %q", model.lastNotice)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Language != domain.LanguageSimplifiedChinese {
		t.Fatalf("expected persisted language zh-CN, got %q", cfg.Language)
	}
}

func TestHintMessageMentionsInspectorTabs(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	got := model.hintMessage()
	if !strings.Contains(got, "Tab profiles/stacks") {
		t.Fatalf("expected focus-switch hint, got %q", got)
	}
	if !strings.Contains(got, "h/l details/logs") {
		t.Fatalf("expected inspector tab hint, got %q", got)
	}
	if !strings.Contains(got, "i import drafts") {
		t.Fatalf("expected import hint, got %q", got)
	}
	if !strings.Contains(got, "Enter start tunnel") {
		t.Fatalf("expected explicit Enter action, got %q", got)
	}
	if !strings.Contains(got, "r restart tunnel") {
		t.Fatalf("expected restart hint, got %q", got)
	}
	if !strings.Contains(got, "g reload config") {
		t.Fatalf("expected reload hint, got %q", got)
	}
}

func TestHintMessageMentionsLogFilteringWhenLogsTabActive(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, inspectorTab: inspectorTabLogs}
	got := model.hintMessage()
	if !strings.Contains(got, "/ filter logs") {
		t.Fatalf("expected logs filter hint, got %q", got)
	}
	if !strings.Contains(got, "f follow/pause") || !strings.Contains(got, "n/N hits") {
		t.Fatalf("expected expanded log navigation hints, got %q", got)
	}
}

func TestHintMessageMentionsImportAndSampleWhenWorkspaceEmpty(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	got := model.hintMessage()
	for _, snippet := range []string{"i import drafts", "s sample config", "a profile preset"} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected %q in empty-workspace hint, got %q", snippet, got)
		}
	}
}

func TestHandleWorkspaceKeyUsesAForProfilePresetMode(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}, nil, nil)
	if !handled {
		t.Fatal("expected a profile-preset key to be handled")
	}
	if next.createPresetMode != createPresetProfile {
		t.Fatalf("expected profile preset mode, got %v", next.createPresetMode)
	}
	if !next.hasStatusLine() {
		t.Fatal("expected preset mode to reserve the status line")
	}
	if !strings.Contains(next.renderStatusLine(120), "Create profile preset") {
		t.Fatalf("expected preset prompt banner, got %q", next.renderStatusLine(120))
	}
}

func TestHandleWorkspaceKeyUsesShiftAForStackPresetMode(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}}, nil, nil)
	if !handled {
		t.Fatal("expected A stack-preset key to be handled")
	}
	if next.createPresetMode != createPresetStack {
		t.Fatalf("expected stack preset mode, got %v", next.createPresetMode)
	}
	if !strings.Contains(next.renderStatusLine(120), "Create stack preset") {
		t.Fatalf("expected stack preset banner, got %q", next.renderStatusLine(120))
	}
}

func TestRestartSelectionRestartsSelectedProfile(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}

	runtime := newStubRuntimeController()
	runtime.states["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
		PID:         1,
	}

	service, err := app.NewService(cfg, runtime, app.WithPortChecker(stubPortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	model = model.restartSelection(profiles, stacks)

	if model.lastError != "" {
		t.Fatalf("expected no restart error, got %q", model.lastError)
	}
	if !strings.Contains(model.lastNotice, "Restarted profile prod-db.") {
		t.Fatalf("expected restart notice, got %q", model.lastNotice)
	}
	if len(runtime.stoppedNames) != 1 || runtime.stoppedNames[0] != "prod-db" {
		t.Fatalf("expected prod-db to be stopped before restart, got %#v", runtime.stoppedNames)
	}
	state, ok := runtime.states["prod-db"]
	if !ok || state.Status != domain.TunnelStatusRunning {
		t.Fatalf("expected prod-db to be running after restart, got %#v", state)
	}
}

func TestUpdateUsesLowercaseRForRestart(t *testing.T) {
	t.Parallel()

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}

	runtime := newStubRuntimeController()
	runtime.states["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
		PID:         1,
	}

	service, err := app.NewService(cfg, runtime, app.WithPortChecker(stubPortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	nextModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	updated := nextModel.(Model)

	if !strings.Contains(updated.lastNotice, "Restarted profile prod-db.") {
		t.Fatalf("expected lowercase r to restart selected profile, got notice %q", updated.lastNotice)
	}
}

func TestHandleWorkspaceKeyUsesGForReload(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, configPath: configPath}
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}, nil, nil)
	if !handled {
		t.Fatal("expected g reload key to be handled")
	}
	if !strings.Contains(next.lastNotice, "Reloaded config from disk.") {
		t.Fatalf("expected reload notice, got %q", next.lastNotice)
	}
	if got := len(next.service.ProfileViews()); got != 2 {
		t.Fatalf("expected reload to refresh profiles from disk, got %d", got)
	}
}

func TestHandleWorkspaceKeyUsesIForImportMode(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}, nil, nil)
	if !handled {
		t.Fatal("expected i import key to be handled")
	}
	if !next.importMode {
		t.Fatal("expected import mode to be enabled")
	}
}

func TestHandleWorkspaceKeyUsesSForSampleConfigWhenEmpty(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, configPath: configPath}
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}, nil, nil)
	if !handled {
		t.Fatal("expected s sample key to be handled")
	}
	if !strings.Contains(next.lastNotice, "Initialized sample config") {
		t.Fatalf("expected sample-config notice, got %q", next.lastNotice)
	}
	if got := len(next.service.ProfileViews()); got != 2 {
		t.Fatalf("expected sample config to create 2 profiles, got %d", got)
	}
}

func TestHandleWorkspaceKeyUsesEForFormEditor(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected e form-edit key to be handled")
	}
	if next.editor == nil || next.editor.kind != formEditorProfile {
		t.Fatalf("expected profile editor to open, got %#v", next.editor)
	}
}

func TestCreateStarterProfileDraftFocusesRecommendedField(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: configPath,
	}
	model = model.createStarterProfileDraft()

	field, ok := model.editor.currentField()
	if !ok {
		t.Fatal("expected active editor field")
	}
	if field.key != editorFieldHost {
		t.Fatalf("expected starter draft to focus SSH Host, got %q", field.key)
	}
	if guide := model.editorGuideLine(); !strings.Contains(guide, "SSH Host") {
		t.Fatalf("expected guide line to mention SSH Host, got %q", guide)
	}
}

func TestBeginProfileEditorGuidesImportedKubernetesDraftToResource(t *testing.T) {
	t.Parallel()

	model := Model{}
	model = model.beginProfileEditor(domain.Profile{
		Name:      "dev-cluster",
		Type:      domain.TunnelTypeKubernetesPortForward,
		LocalPort: 18080,
		Labels:    []string{"draft", "imported", "kube-context"},
		Kubernetes: &domain.Kubernetes{
			Context:      "dev-cluster",
			Namespace:    "backend",
			ResourceType: "service",
			Resource:     "change-me",
			RemotePort:   80,
		},
	}, "dev-cluster")

	field, ok := model.editor.currentField()
	if !ok {
		t.Fatal("expected active editor field")
	}
	if field.key != editorFieldResource {
		t.Fatalf("expected imported kube draft to focus Resource, got %q", field.key)
	}
	if guide := model.editorGuideLine(); !strings.Contains(guide, "Resource") {
		t.Fatalf("expected guide line to mention Resource, got %q", guide)
	}
}

func TestCreateStarterStackDraftFocusesFirstMemberField(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: configPath,
	}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	model = model.createStarterStackDraft(profiles, stacks)

	field, ok := model.editor.currentField()
	if !ok {
		t.Fatal("expected active editor field")
	}
	if field.key != stackMemberFieldKey(0) {
		t.Fatalf("expected starter stack draft to focus first member, got %q", field.key)
	}
	if guide := model.editorGuideLine(); !strings.Contains(guide, "Member 1") {
		t.Fatalf("expected guide line to mention Member 1, got %q", guide)
	}
}

func TestSaveProfileEditorPersistsUpdatedFields(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: configPath,
	}
	model = model.createStarterProfileDraft()
	model.editor.values[editorFieldHost] = "bastion-staging"
	model.editor.values[editorFieldRemoteHost] = "staging-db.internal"
	model.editor.values[editorFieldRemotePort] = "15432"

	model = model.saveActiveEditor()

	if model.lastError != "" {
		t.Fatalf("expected no error after save, got %q", model.lastError)
	}
	if model.editor != nil {
		t.Fatalf("expected editor to close after save, got %#v", model.editor)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles[0].SSH == nil || cfg.Profiles[0].SSH.Host != "bastion-staging" || cfg.Profiles[0].SSH.RemoteHost != "staging-db.internal" {
		t.Fatalf("unexpected saved profile: %#v", cfg.Profiles[0])
	}
}

func TestStackEditorSupportsDynamicMemberAddMoveRemoveAndSaveOrder(t *testing.T) {
	t.Parallel()

	model := Model{
		editor: newStackEditorState(domain.Stack{
			Name:     "backend-dev",
			Profiles: []string{"prod-db", "api-debug"},
		}, "backend-dev", domain.LanguageEnglish),
	}
	model.editor.focusFieldByKey(stackMemberFieldKey(0))

	next, _, handled := model.handleEditorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if !handled {
		t.Fatal("expected ] to reorder stack members")
	}
	members, _ := next.editor.stackMemberSnapshot()
	if got := strings.Join(members, ","); got != "api-debug,prod-db" {
		t.Fatalf("expected reordered members api-debug,prod-db, got %q", got)
	}

	next, _, handled = next.handleEditorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	if !handled {
		t.Fatal("expected + to add a stack member field")
	}
	members, _ = next.editor.stackMemberSnapshot()
	if len(members) != 3 || members[2] != "" {
		t.Fatalf("expected a blank member field to be added, got %#v", members)
	}

	next.editor.setValue(stackMemberFieldKey(2), "worker-debug")
	next, _, handled = next.handleEditorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if !handled {
		t.Fatal("expected [ to move the new member upward")
	}
	stack, err := next.editor.stackFromValues(true)
	if err != nil {
		t.Fatalf("expected stack editor values to remain valid, got %v", err)
	}
	if got := strings.Join(stack.Profiles, ","); got != "api-debug,worker-debug,prod-db" {
		t.Fatalf("expected saved member order api-debug,worker-debug,prod-db, got %q", got)
	}

	next, _, handled = next.handleEditorKey(tea.KeyMsg{Type: tea.KeyCtrlX})
	if !handled {
		t.Fatal("expected Ctrl+X to remove the selected member")
	}
	members, _ = next.editor.stackMemberSnapshot()
	if got := strings.Join(members, ","); got != "api-debug,prod-db" {
		t.Fatalf("expected removed member list api-debug,prod-db, got %q", got)
	}
}

func TestStackEditorCommaSplitsIntoNextMemberRow(t *testing.T) {
	t.Parallel()

	model := Model{
		editor: newStackEditorState(domain.Stack{
			Name:     "backend-dev",
			Profiles: []string{"prod-db"},
		}, "backend-dev", domain.LanguageEnglish),
	}
	model.editor.focusFieldByKey(stackMemberFieldKey(0))
	model.editor.setValue(stackMemberFieldKey(0), "prod-db")
	model.editor.moveCursorToEnd(stackMemberFieldKey(0))

	next, _, handled := model.handleEditorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{','}})
	if !handled {
		t.Fatal("expected comma split to be handled")
	}

	members, _ := next.editor.stackMemberSnapshot()
	if got := strings.Join(members, "|"); got != "prod-db|" {
		t.Fatalf("expected split members to add a blank row, got %q", got)
	}
	field, ok := next.editor.currentField()
	if !ok || field.key != stackMemberFieldKey(1) {
		t.Fatalf("expected focus to move to the new member row, got %#v", field)
	}
}

func TestStackEditorPastedMemberListExpandsRows(t *testing.T) {
	t.Parallel()

	model := Model{
		editor: newStackEditorState(domain.Stack{
			Name:     "backend-dev",
			Profiles: []string{""},
		}, "backend-dev", domain.LanguageEnglish),
	}
	model.editor.focusFieldByKey(stackMemberFieldKey(0))

	next, _, handled := model.handleEditorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("prod-db, api-debug\nworker-debug")})
	if !handled {
		t.Fatal("expected pasted member list to be handled")
	}

	members, _ := next.editor.stackMemberSnapshot()
	if got := strings.Join(members, ","); got != "prod-db,api-debug,worker-debug" {
		t.Fatalf("expected pasted list to expand into rows, got %q", got)
	}
	if help := next.editorHelpLine(); !strings.Contains(help, "paste comma/newline lists to expand") {
		t.Fatalf("expected member help to mention batch paste, got %q", help)
	}
}

func TestSaveStackEditorDedupesMembersAndReportsIt(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:    service,
		configPath: configPath,
		editor: newStackEditorState(domain.Stack{
			Name:     "backend-dev",
			Labels:   []string{"draft"},
			Profiles: []string{"prod-db", "api-debug", "prod-db"},
		}, "backend-dev", domain.LanguageEnglish),
	}

	model = model.saveActiveEditor()

	if model.lastError != "" {
		t.Fatalf("expected no save error, got %q", model.lastError)
	}
	if !strings.Contains(model.lastNotice, "removed 1 duplicate member entries") {
		t.Fatalf("expected duplicate-removal notice, got %q", model.lastNotice)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	var saved domain.Stack
	found := false
	for _, stack := range cfg.Stacks {
		if stack.Name == "backend-dev" {
			saved = stack
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected saved stack backend-dev")
	}
	if got := strings.Join(saved.Profiles, ","); got != "prod-db,api-debug" {
		t.Fatalf("expected deduped members prod-db,api-debug, got %q", got)
	}
}

func TestEditorHelpLineShowsAvailableProfilesForStackMembers(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service: service,
		editor: newStackEditorState(domain.Stack{
			Name:     "draft-stack",
			Labels:   []string{"draft"},
			Profiles: []string{"prod-db"},
		}, "draft-stack", domain.LanguageEnglish),
	}

	help := model.editorHelpLine()
	if !strings.Contains(help, "Available profiles:") {
		t.Fatalf("expected available-profile hint, got %q", help)
	}
	if !strings.Contains(help, "prod-db") || !strings.Contains(help, "api-debug") {
		t.Fatalf("expected available profile names in help, got %q", help)
	}
}

func TestHandleWorkspaceKeyUsesPToOpenSelectedStackMember(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:         service,
		focus:           focusStacks,
		selectedProfile: 1,
		selectedStack:   0,
		filterQuery:     "backend",
		filterMode:      true,
	}

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected p key to be handled for stacks")
	}
	if next.focus != focusProfiles {
		t.Fatalf("expected focus to switch to profiles, got %v", next.focus)
	}
	if next.filterQuery != "" || next.filterMode {
		t.Fatalf("expected filter to clear when opening member, got query=%q mode=%v", next.filterQuery, next.filterMode)
	}
	if got := next.service.ProfileViews()[next.selectedProfile].Profile.Name; got != "prod-db" {
		t.Fatalf("expected first stack member prod-db to be focused, got %q", got)
	}
	if !strings.Contains(next.lastNotice, "Opened member profile prod-db from stack backend-dev.") {
		t.Fatalf("expected open-member notice, got %q", next.lastNotice)
	}
}

func TestHandleWorkspaceKeyUsesPToOpenCurrentlySelectedStackMember(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:             service,
		focus:               focusStacks,
		selectedStack:       0,
		selectedStackMember: 1,
		inspectorTab:        inspectorTabDetails,
	}

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	next, _, handled := model.handleWorkspaceKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected p key to be handled for the selected stack member")
	}
	if got := next.service.ProfileViews()[next.selectedProfile].Profile.Name; got != "api-debug" {
		t.Fatalf("expected selected member api-debug to be opened, got %q", got)
	}
	if !strings.Contains(next.lastNotice, "Opened member profile api-debug from stack backend-dev.") {
		t.Fatalf("expected selected-member notice, got %q", next.lastNotice)
	}
}

func TestHandleInspectorKeyControlsSelectedStackMember(t *testing.T) {
	t.Parallel()

	runtime := newStubRuntimeController()
	runtime.states["api-debug"] = domain.RuntimeState{
		ProfileName: "api-debug",
		Status:      domain.TunnelStatusRunning,
		PID:         42,
	}

	service, err := app.NewService(storage.SampleConfig(), runtime, app.WithPortChecker(stubPortChecker{}))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:             service,
		focus:               focusStacks,
		selectedStack:       0,
		selectedStackMember: 1,
		inspectorTab:        inspectorTabDetails,
		width:               140,
		height:              30,
	}

	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")

	next, handled := model.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected S to toggle the selected stack member")
	}
	if len(runtime.stoppedNames) == 0 || runtime.stoppedNames[len(runtime.stoppedNames)-1] != "api-debug" {
		t.Fatalf("expected api-debug to be stopped, got %#v", runtime.stoppedNames)
	}
	if !strings.Contains(next.lastNotice, "api-debug") {
		t.Fatalf("expected member toggle notice to mention api-debug, got %q", next.lastNotice)
	}

	profiles = filterProfileViews(service.ProfileViews(), "")
	stacks = filterStackViews(service.StackViews(), "")
	next, handled = next.handleInspectorKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}, profiles, stacks)
	if !handled {
		t.Fatal("expected R to restart the selected stack member")
	}
	if state, ok := runtime.states["api-debug"]; !ok || state.Status != domain.TunnelStatusRunning {
		t.Fatalf("expected api-debug to be running after restart, got %#v", state)
	}
	if !strings.Contains(next.lastNotice, "Restarted member api-debug in stack backend-dev.") {
		t.Fatalf("expected restart notice for selected member, got %q", next.lastNotice)
	}
}

func TestHandleImportKeyImportsSSHDrafts(t *testing.T) {
	homeDir := t.TempDir()
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}
	sshConfigPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(sshConfigPath, []byte("Host bastion-prod\n  HostName bastion.internal\n  User deploy\n  Port 2222\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}
	t.Setenv("HOME", homeDir)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, configPath: configPath, importMode: true}
	next, handled := model.handleImportKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if !handled {
		t.Fatal("expected s SSH import key to be handled")
	}
	if next.importMode {
		t.Fatal("expected import mode to exit after SSH import")
	}
	if !strings.Contains(next.lastNotice, "Imported SSH drafts") {
		t.Fatalf("expected SSH import notice, got %q", next.lastNotice)
	}
	if got := len(next.service.ProfileViews()); got != 1 {
		t.Fatalf("expected 1 imported profile, got %d", got)
	}
	if got := next.service.ProfileViews()[0].Profile.Name; got != "bastion-prod" {
		t.Fatalf("expected imported profile bastion-prod, got %q", got)
	}
}

func TestHandleCreatePresetKeyCreatesProfileDraftVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		key        rune
		wantType   domain.TunnelType
		verifyFunc func(t *testing.T, profile domain.Profile)
	}{
		{
			name:     "ssh local",
			key:      'l',
			wantType: domain.TunnelTypeSSHLocal,
			verifyFunc: func(t *testing.T, profile domain.Profile) {
				t.Helper()
				if profile.SSH == nil || profile.SSH.Host != "example-bastion" {
					t.Fatalf("expected ssh-local preset fields, got %#v", profile)
				}
			},
		},
		{
			name:     "ssh remote",
			key:      'r',
			wantType: domain.TunnelTypeSSHRemote,
			verifyFunc: func(t *testing.T, profile domain.Profile) {
				t.Helper()
				if profile.SSHRemote == nil || profile.SSHRemote.BindPort != 9000 {
					t.Fatalf("expected ssh-remote preset fields, got %#v", profile)
				}
			},
		},
		{
			name:     "ssh dynamic",
			key:      'd',
			wantType: domain.TunnelTypeSSHDynamic,
			verifyFunc: func(t *testing.T, profile domain.Profile) {
				t.Helper()
				if profile.SSHDynamic == nil || profile.LocalPort != 1080 {
					t.Fatalf("expected ssh-dynamic preset fields, got %#v", profile)
				}
			},
		},
		{
			name:     "kubernetes",
			key:      'k',
			wantType: domain.TunnelTypeKubernetesPortForward,
			verifyFunc: func(t *testing.T, profile domain.Profile) {
				t.Helper()
				if profile.Kubernetes == nil || profile.Kubernetes.Resource != "change-me" {
					t.Fatalf("expected kubernetes preset fields, got %#v", profile)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configPath := filepath.Join(t.TempDir(), "config.yaml")
			service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
			if err != nil {
				t.Fatalf("new service: %v", err)
			}

			model := Model{
				service:          service,
				configPath:       configPath,
				createPresetMode: createPresetProfile,
			}

			next, handled := model.handleCreatePresetKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}}, nil, nil)
			if !handled {
				t.Fatal("expected preset key to be handled")
			}
			if next.createPresetMode != createPresetNone {
				t.Fatalf("expected preset mode to exit, got %v", next.createPresetMode)
			}
			if next.editor == nil || next.editor.kind != formEditorProfile {
				t.Fatalf("expected profile editor to open, got %#v", next.editor)
			}

			views := next.service.ProfileViews()
			if len(views) != 1 {
				t.Fatalf("expected 1 profile view, got %d", len(views))
			}
			profile := views[next.selectedProfile].Profile
			if profile.Type != tt.wantType {
				t.Fatalf("expected type %q, got %#v", tt.wantType, profile)
			}
			tt.verifyFunc(t, profile)

			cfg, err := storage.LoadConfig(configPath)
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			if len(cfg.Profiles) != 1 {
				t.Fatalf("expected persisted preset profile, got %d", len(cfg.Profiles))
			}
			if cfg.Profiles[0].Type != tt.wantType {
				t.Fatalf("expected persisted type %q, got %#v", tt.wantType, cfg.Profiles[0])
			}
		})
	}
}

func TestHandleCreatePresetKeyCreatesStackPresetVariants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		key       rune
		newModel  func(t *testing.T, configPath string) Model
		wantNames string
		wantLabel string
	}{
		{
			name: "selection",
			key:  's',
			newModel: func(t *testing.T, configPath string) Model {
				t.Helper()
				service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
				if err != nil {
					t.Fatalf("new service: %v", err)
				}
				return Model{
					service:          service,
					configPath:       configPath,
					selectedProfile:  1,
					createPresetMode: createPresetStack,
				}
			},
			wantNames: "api-debug",
			wantLabel: "Created starter stack",
		},
		{
			name: "visible",
			key:  'v',
			newModel: func(t *testing.T, configPath string) Model {
				t.Helper()
				service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
				if err != nil {
					t.Fatalf("new service: %v", err)
				}
				return Model{
					service:          service,
					configPath:       configPath,
					createPresetMode: createPresetStack,
				}
			},
			wantNames: "prod-db,api-debug",
			wantLabel: "Created visible-profiles stack",
		},
		{
			name: "running",
			key:  'r',
			newModel: func(t *testing.T, configPath string) Model {
				t.Helper()
				runtime := newStubRuntimeController()
				runtime.states["api-debug"] = domain.RuntimeState{
					ProfileName: "api-debug",
					Status:      domain.TunnelStatusRunning,
					PID:         7,
				}

				service, err := app.NewService(storage.SampleConfig(), runtime)
				if err != nil {
					t.Fatalf("new service: %v", err)
				}
				return Model{
					service:          service,
					configPath:       configPath,
					createPresetMode: createPresetStack,
				}
			},
			wantNames: "api-debug",
			wantLabel: "Created running-profiles stack",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configPath := filepath.Join(t.TempDir(), "config.yaml")
			model := tt.newModel(t, configPath)
			profiles := filterProfileViews(model.service.ProfileViews(), model.filterQuery)
			stacks := filterStackViews(model.service.StackViews(), model.filterQuery)

			next, handled := model.handleCreatePresetKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}}, profiles, stacks)
			if !handled {
				t.Fatal("expected stack preset key to be handled")
			}
			if next.createPresetMode != createPresetNone {
				t.Fatalf("expected preset mode to exit, got %v", next.createPresetMode)
			}
			if next.editor == nil || next.editor.kind != formEditorStack {
				t.Fatalf("expected stack editor to open, got %#v", next.editor)
			}
			if !strings.Contains(next.lastNotice, tt.wantLabel) {
				t.Fatalf("expected creation notice %q, got %q", tt.wantLabel, next.lastNotice)
			}

			stackViews := next.service.StackViews()
			if len(stackViews) < 2 {
				t.Fatalf("expected preset stack to be persisted, got %d stacks", len(stackViews))
			}
			selected := stackViews[next.selectedStack].Stack
			if got := strings.Join(selected.Profiles, ","); got != tt.wantNames {
				t.Fatalf("expected stack members %q, got %q", tt.wantNames, got)
			}

			cfg, err := storage.LoadConfig(configPath)
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			if len(cfg.Stacks) < 2 {
				t.Fatalf("expected persisted preset stack, got %d stacks", len(cfg.Stacks))
			}
		})
	}
}

func TestHandleMouseClickSelectsProfileAndStackRows(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, width: 140, height: 30}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	layout := model.mouseLayout(profiles, stacks)

	profileMsg := tea.MouseMsg{
		X:      panelContentX(layout.profiles.panel) + 1,
		Y:      panelBodyStartY(layout.profiles.panel) + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled := model.handleMouse(profileMsg, profiles, stacks)
	if !handled {
		t.Fatal("expected profile row click to be handled")
	}
	if next.focus != focusProfiles || next.selectedProfile != 1 {
		t.Fatalf("expected second profile to be selected, got focus=%v selectedProfile=%d", next.focus, next.selectedProfile)
	}

	stackMsg := tea.MouseMsg{
		X:      panelContentX(layout.stacks.panel) + 1,
		Y:      panelBodyStartY(layout.stacks.panel),
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled = next.handleMouse(stackMsg, profiles, stacks)
	if !handled {
		t.Fatal("expected stack row click to be handled")
	}
	if next.focus != focusStacks || next.selectedStack != 0 {
		t.Fatalf("expected stack to be focused and selected, got focus=%v selectedStack=%d", next.focus, next.selectedStack)
	}
}

func TestHandleMouseClickSwitchesInspectorTab(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, width: 140, height: 30}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	layout := model.mouseLayout(profiles, stacks)

	var logsRegion mouseInspectorTabRegion
	found := false
	for _, region := range layout.inspectorTabs {
		if region.tab == inspectorTabLogs {
			logsRegion = region
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected logs tab region")
	}

	msg := tea.MouseMsg{
		X:      logsRegion.rect.x + 1,
		Y:      logsRegion.rect.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled := model.handleMouse(msg, profiles, stacks)
	if !handled {
		t.Fatal("expected tab click to be handled")
	}
	if next.inspectorTab != inspectorTabLogs {
		t.Fatalf("expected logs tab to become active, got %v", next.inspectorTab)
	}
}

func TestHandleMouseWheelScrollsInspector(t *testing.T) {
	t.Parallel()

	logs := make([]domain.LogEntry, 0, 40)
	base := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		logs = append(logs, domain.LogEntry{
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Source:    domain.LogSourceStdout,
			Message:   fmt.Sprintf("log line %02d", i),
		})
	}

	runtime := newStubRuntimeController()
	runtime.states["prod-db"] = domain.RuntimeState{
		ProfileName: "prod-db",
		Status:      domain.TunnelStatusRunning,
		RecentLogs:  logs,
	}

	cfg := domain.Config{
		Version: domain.CurrentConfigVersion,
		Profiles: []domain.Profile{
			{
				Name:      "prod-db",
				Type:      domain.TunnelTypeSSHLocal,
				LocalPort: 15432,
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
		},
	}

	service, err := app.NewService(cfg, runtime)
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, width: 140, height: 30, inspectorTab: inspectorTabLogs}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	layout := model.mouseLayout(profiles, stacks)

	msg := tea.MouseMsg{
		X:      panelContentX(layout.inspector) + 1,
		Y:      panelBodyStartY(layout.inspector) + 2,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	}
	next, handled := model.handleMouse(msg, profiles, stacks)
	if !handled {
		t.Fatal("expected wheel event to be handled")
	}
	if next.inspectorScroll <= 0 {
		t.Fatalf("expected inspector scroll to increase, got %d", next.inspectorScroll)
	}
}

func TestHandleMouseClickActivatesHeaderFilter(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, width: 140, height: 30}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	layout := model.mouseLayout(profiles, stacks)

	msg := tea.MouseMsg{
		X:      layout.headerFilter.x + 1,
		Y:      layout.headerFilter.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled := model.handleMouse(msg, profiles, stacks)
	if !handled {
		t.Fatal("expected filter click to be handled")
	}
	if !next.filterMode || next.filterScope != filterScopeList {
		t.Fatalf("expected list filter mode to activate, got mode=%v scope=%v", next.filterMode, next.filterScope)
	}
}

func TestHandleMouseClickTriggersImportPromptAction(t *testing.T) {
	homeDir := t.TempDir()
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}
	sshConfigPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(sshConfigPath, []byte("Host bastion-prod\n  HostName bastion.internal\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}
	t.Setenv("HOME", homeDir)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{service: service, configPath: configPath, importMode: true, width: 140, height: 30}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	layout := model.mouseLayout(profiles, stacks)

	if len(layout.importActions) == 0 {
		t.Fatal("expected import actions to be clickable")
	}

	msg := tea.MouseMsg{
		X:      layout.importActions[0].rect.x + 1,
		Y:      layout.importActions[0].rect.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled := model.handleMouse(msg, profiles, stacks)
	if !handled {
		t.Fatal("expected import click to be handled")
	}
	if next.importMode {
		t.Fatal("expected import mode to exit after click")
	}
	if len(next.service.ProfileViews()) != 1 {
		t.Fatalf("expected 1 imported profile, got %d", len(next.service.ProfileViews()))
	}
}

func TestHandleMouseClickTriggersPresetPromptAction(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	service, err := app.NewService(domain.DefaultConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:          service,
		configPath:       configPath,
		createPresetMode: createPresetProfile,
		width:            140,
		height:           30,
	}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	layout := model.mouseLayout(profiles, stacks)

	if len(layout.presetActions) == 0 {
		t.Fatal("expected preset actions to be clickable")
	}

	msg := tea.MouseMsg{
		X:      layout.presetActions[0].rect.x + 1,
		Y:      layout.presetActions[0].rect.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled := model.handleMouse(msg, profiles, stacks)
	if !handled {
		t.Fatal("expected preset click to be handled")
	}
	if next.createPresetMode != createPresetNone {
		t.Fatalf("expected preset mode to exit after click, got %v", next.createPresetMode)
	}
	if next.editor == nil || next.editor.kind != formEditorProfile {
		t.Fatalf("expected profile editor to open, got %#v", next.editor)
	}
	if len(next.service.ProfileViews()) != 1 {
		t.Fatalf("expected 1 created profile, got %d", len(next.service.ProfileViews()))
	}
	if next.service.ProfileViews()[0].Profile.Type != domain.TunnelTypeSSHLocal {
		t.Fatalf("expected first preset click to create ssh-local profile, got %#v", next.service.ProfileViews()[0].Profile)
	}
}

func TestHandleMouseClickOnStackMemberSelectsMember(t *testing.T) {
	t.Parallel()

	service, err := app.NewService(storage.SampleConfig(), newStubRuntimeController())
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	model := Model{
		service:       service,
		width:         140,
		height:        30,
		focus:         focusStacks,
		selectedStack: 0,
		inspectorTab:  inspectorTabDetails,
	}
	profiles := filterProfileViews(service.ProfileViews(), "")
	stacks := filterStackViews(service.StackViews(), "")
	layout := model.mouseLayout(profiles, stacks)

	if len(layout.stackMembers) < 2 {
		t.Fatal("expected stack member rows to be clickable")
	}

	msg := tea.MouseMsg{
		X:      layout.stackMembers[1].rect.x + 1,
		Y:      layout.stackMembers[1].rect.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled := model.handleMouse(msg, profiles, stacks)
	if !handled {
		t.Fatal("expected stack member click to be handled")
	}
	if next.focus != focusStacks {
		t.Fatalf("expected member click to keep stack focus, got %v", next.focus)
	}
	if next.selectedStackMember != 1 {
		t.Fatalf("expected clicked member index 1 to be selected, got %d", next.selectedStackMember)
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
	clearedLogs   []string
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

func (s *stubRuntimeController) ClearLogs(name string) error {
	s.clearedLogs = append(s.clearedLogs, name)
	state, exists := s.states[name]
	if !exists {
		return nil
	}
	state.RecentLogs = nil
	s.states[name] = state
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

func containsLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}

type stubPortChecker struct{}

func (stubPortChecker) CheckLocalPort(port int) error {
	return nil
}

func findProfileByName(profiles []domain.Profile, name string) (domain.Profile, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return domain.Profile{}, false
}

func findStackByName(stacks []domain.Stack, name string) (domain.Stack, bool) {
	for _, stack := range stacks {
		if stack.Name == name {
			return stack, true
		}
	}
	return domain.Stack{}, false
}
