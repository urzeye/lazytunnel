package kubernetes

import (
	"reflect"
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func TestBuildProcessSpecBuildsKubectlCommand(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name:      "api-debug",
		Type:      domain.TunnelTypeKubernetesPortForward,
		LocalPort: 8080,
		Restart: domain.RestartPolicy{
			Enabled:        true,
			InitialBackoff: "2s",
		},
		Kubernetes: &domain.Kubernetes{
			Context:      "dev-cluster",
			Namespace:    "backend",
			ResourceType: "service",
			Resource:     "api",
			RemotePort:   80,
		},
	})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	if spec.Command != "kubectl" {
		t.Fatalf("expected kubectl command, got %q", spec.Command)
	}

	wantArgs := []string{
		"--context", "dev-cluster",
		"--namespace", "backend",
		"port-forward",
		"service/api",
		"8080:80",
		"--address", "127.0.0.1",
	}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Fatalf("unexpected args: %#v", spec.Args)
	}
}

func TestBuildProcessSpecRejectsWrongProfileType(t *testing.T) {
	t.Parallel()

	_, err := BuildProcessSpec(domain.Profile{
		Name:      "prod-db",
		Type:      domain.TunnelTypeSSHLocal,
		LocalPort: 5432,
		SSH: &domain.SSHLocal{
			Host:       "bastion",
			RemoteHost: "db.internal",
			RemotePort: 5432,
		},
	})
	if err == nil {
		t.Fatal("expected wrong type error")
	}
}
