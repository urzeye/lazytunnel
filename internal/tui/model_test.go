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
	}, newStubRuntimeController())
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
	}, newStubRuntimeController())
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

	for _, snippet := range []string{"import drafts", "sample config", "draft profile", "edit config", "reload config"} {
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
	if !strings.Contains(got, "Home latest End oldest") {
		t.Fatalf("expected log navigation hint, got %q", got)
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
	for _, snippet := range []string{"i import drafts", "s sample config", "a new tunnel draft"} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected %q in empty-workspace hint, got %q", snippet, got)
		}
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

func TestHandleMouseClickOnStackMemberFocusesProfile(t *testing.T) {
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

	if len(layout.stackMembers) == 0 {
		t.Fatal("expected stack member rows to be clickable")
	}

	msg := tea.MouseMsg{
		X:      layout.stackMembers[0].rect.x + 1,
		Y:      layout.stackMembers[0].rect.y,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	}
	next, handled := model.handleMouse(msg, profiles, stacks)
	if !handled {
		t.Fatal("expected stack member click to be handled")
	}
	if next.focus != focusProfiles {
		t.Fatalf("expected member click to switch focus to profiles, got %v", next.focus)
	}
	if got := next.service.ProfileViews()[next.selectedProfile].Profile.Name; got != "prod-db" {
		t.Fatalf("expected clicked member prod-db to be focused, got %q", got)
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
