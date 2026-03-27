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
