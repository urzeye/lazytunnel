package ssh

import (
	"reflect"
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func TestBuildProcessSpecBuildsSSHCommand(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name:      "prod-db",
		Type:      domain.TunnelTypeSSHLocal,
		LocalPort: 5432,
		Restart: domain.RestartPolicy{
			Enabled:        true,
			InitialBackoff: "2s",
		},
		SSH: &domain.SSHLocal{
			Host:       "bastion-prod",
			RemoteHost: "db.internal",
			RemotePort: 5432,
		},
	})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	if spec.Command != "ssh" {
		t.Fatalf("expected ssh command, got %q", spec.Command)
	}

	wantArgs := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-L", "5432:db.internal:5432",
		"bastion-prod",
	}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Fatalf("unexpected args: %#v", spec.Args)
	}
}

func TestBuildProcessSpecRejectsWrongProfileType(t *testing.T) {
	t.Parallel()

	_, err := BuildProcessSpec(domain.Profile{
		Name:      "api-debug",
		Type:      domain.TunnelTypeKubernetesPortForward,
		LocalPort: 8080,
		Kubernetes: &domain.Kubernetes{
			ResourceType: "service",
			Resource:     "api",
			RemotePort:   80,
		},
	})
	if err == nil {
		t.Fatal("expected wrong type error")
	}
}

func TestBuildProcessSpecBuildsSSHRemoteCommand(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name: "public-api",
		Type: domain.TunnelTypeSSHRemote,
		Restart: domain.RestartPolicy{
			Enabled: true,
		},
		SSHRemote: &domain.SSHRemote{
			Host:        "bastion-prod",
			BindAddress: "0.0.0.0",
			BindPort:    9000,
			TargetHost:  "127.0.0.1",
			TargetPort:  8080,
		},
	})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	wantArgs := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-R", "0.0.0.0:9000:127.0.0.1:8080",
		"bastion-prod",
	}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Fatalf("unexpected args: %#v", spec.Args)
	}
}

func TestBuildProcessSpecBuildsSSHDynamicCommand(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name:      "dev-socks",
		Type:      domain.TunnelTypeSSHDynamic,
		LocalPort: 1080,
		Restart: domain.RestartPolicy{
			Enabled: true,
		},
		SSHDynamic: &domain.SSHDynamic{
			Host:        "bastion-prod",
			BindAddress: "127.0.0.1",
		},
	})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	wantArgs := []string{
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-D", "127.0.0.1:1080",
		"bastion-prod",
	}
	if !reflect.DeepEqual(spec.Args, wantArgs) {
		t.Fatalf("unexpected args: %#v", spec.Args)
	}
}
