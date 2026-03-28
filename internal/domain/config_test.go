package domain

import (
	"strings"
	"testing"
)

func TestConfigValidateAcceptsValidConfig(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Version: CurrentConfigVersion,
		Profiles: []Profile{
			{
				Name:      "prod-db",
				Type:      TunnelTypeSSHLocal,
				LocalPort: 5432,
				Restart: RestartPolicy{
					Enabled:        true,
					InitialBackoff: "2s",
					MaxBackoff:     "30s",
				},
				SSH: &SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
			{
				Name:      "api-debug",
				Type:      TunnelTypeKubernetesPortForward,
				LocalPort: 8080,
				Restart: RestartPolicy{
					Enabled:        true,
					InitialBackoff: "2s",
					MaxBackoff:     "30s",
				},
				Kubernetes: &Kubernetes{
					Context:      "dev-cluster",
					Namespace:    "backend",
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
		},
		Stacks: []Stack{
			{
				Name:     "backend-dev",
				Profiles: []string{"prod-db", "api-debug"},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config to be valid, got %v", err)
	}
}

func TestConfigValidateRejectsDuplicateProfileName(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Profiles = []Profile{
		{
			Name:      "dup",
			Type:      TunnelTypeSSHLocal,
			LocalPort: 1001,
			SSH: &SSHLocal{
				Host:       "bastion",
				RemoteHost: "db",
				RemotePort: 5432,
			},
		},
		{
			Name:      "dup",
			Type:      TunnelTypeSSHLocal,
			LocalPort: 1002,
			SSH: &SSHLocal{
				Host:       "bastion",
				RemoteHost: "cache",
				RemotePort: 6379,
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected duplicate profile name error")
	}

	if !strings.Contains(err.Error(), `duplicate profile name "dup"`) {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
}

func TestProfileValidateRejectsIncompleteKubernetesProfile(t *testing.T) {
	t.Parallel()

	profile := Profile{
		Name:      "broken-kube",
		Type:      TunnelTypeKubernetesPortForward,
		LocalPort: 8080,
		Kubernetes: &Kubernetes{
			ResourceType: "statefulset",
			Resource:     "",
			RemotePort:   99999,
		},
	}

	err := profile.Validate()
	if err == nil {
		t.Fatal("expected validation error")
	}

	want := []string{
		"resource_type must be one of pod, service, deployment",
		"resource is required",
		"remote_port must be between 1 and 65535",
	}

	for _, fragment := range want {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("expected %q in %v", fragment, err)
		}
	}
}

func TestConfigSetProfileReplacesExistingProfileByName(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Profiles = []Profile{
		{
			Name:      "prod-db",
			Type:      TunnelTypeSSHLocal,
			LocalPort: 5432,
			SSH: &SSHLocal{
				Host:       "bastion-a",
				RemoteHost: "db.internal",
				RemotePort: 5432,
			},
		},
	}

	created := cfg.SetProfile(Profile{
		Name:      "prod-db",
		Type:      TunnelTypeSSHLocal,
		LocalPort: 15432,
		SSH: &SSHLocal{
			Host:       "bastion-b",
			RemoteHost: "db.internal",
			RemotePort: 5432,
		},
	})
	if created {
		t.Fatal("expected replacement instead of create")
	}

	if got := cfg.Profiles[0].LocalPort; got != 15432 {
		t.Fatalf("expected replaced local port 15432, got %d", got)
	}
}

func TestConfigSetStackAppendsNewStack(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	created := cfg.SetStack(Stack{
		Name:     "backend",
		Profiles: []string{"prod-db"},
	})
	if !created {
		t.Fatal("expected stack to be created")
	}

	if got := len(cfg.Stacks); got != 1 {
		t.Fatalf("expected 1 stack, got %d", got)
	}
}

func TestConfigRemoveProfileDeletesExistingProfile(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Profiles = []Profile{
		{Name: "prod-db"},
		{Name: "api-debug"},
	}

	if removed := cfg.RemoveProfile("prod-db"); !removed {
		t.Fatal("expected profile to be removed")
	}

	if got := len(cfg.Profiles); got != 1 {
		t.Fatalf("expected 1 profile, got %d", got)
	}

	if cfg.Profiles[0].Name != "api-debug" {
		t.Fatalf("expected remaining profile to be api-debug, got %q", cfg.Profiles[0].Name)
	}
}

func TestConfigRemoveProfileFromStacksPrunesEmptyStacks(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Stacks = []Stack{
		{Name: "backend", Profiles: []string{"prod-db", "api-debug"}},
		{Name: "solo", Profiles: []string{"prod-db"}},
	}

	updated, removed := cfg.RemoveProfileFromStacks("prod-db")
	if updated != 2 {
		t.Fatalf("expected 2 updated stacks, got %d", updated)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed stack, got %d", removed)
	}

	if got := len(cfg.Stacks); got != 1 {
		t.Fatalf("expected 1 remaining stack, got %d", got)
	}

	if want := []string{"api-debug"}; strings.Join(cfg.Stacks[0].Profiles, ",") != strings.Join(want, ",") {
		t.Fatalf("expected backend stack to keep %v, got %v", want, cfg.Stacks[0].Profiles)
	}
}

func TestConfigStacksReferencingProfileListsMatches(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Stacks = []Stack{
		{Name: "backend", Profiles: []string{"prod-db", "api-debug"}},
		{Name: "ops", Profiles: []string{"prod-db"}},
	}

	got := cfg.StacksReferencingProfile("prod-db")
	want := []string{"backend", "ops"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestConfigRenameProfileInStacksRewritesReferences(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	cfg.Stacks = []Stack{
		{Name: "backend", Profiles: []string{"prod-db", "api-debug"}},
		{Name: "ops", Profiles: []string{"prod-db"}},
	}

	updated := cfg.RenameProfileInStacks("prod-db", "staging-db")
	if updated != 2 {
		t.Fatalf("expected 2 updated stacks, got %d", updated)
	}

	if got := strings.Join(cfg.Stacks[0].Profiles, ","); got != "staging-db,api-debug" {
		t.Fatalf("unexpected backend profiles: %q", got)
	}
	if got := strings.Join(cfg.Stacks[1].Profiles, ","); got != "staging-db" {
		t.Fatalf("unexpected ops profiles: %q", got)
	}
}
