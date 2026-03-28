package tui

import (
	"testing"
	"time"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
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
