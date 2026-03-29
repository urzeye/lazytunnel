package app

import (
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func TestBuildProcessSpecDispatchesSSHProfile(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name:      "prod-db",
		Type:      domain.TunnelTypeSSHLocal,
		LocalPort: 5432,
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
}

func TestBuildProcessSpecDispatchesKubernetesProfile(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name:      "api-debug",
		Type:      domain.TunnelTypeKubernetesPortForward,
		LocalPort: 8080,
		Kubernetes: &domain.Kubernetes{
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
}

func TestBuildProcessSpecDispatchesSSHRemoteProfile(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name: "public-api",
		Type: domain.TunnelTypeSSHRemote,
		SSHRemote: &domain.SSHRemote{
			Host:       "bastion-prod",
			BindPort:   9000,
			TargetHost: "127.0.0.1",
			TargetPort: 8080,
		},
	})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	if spec.Command != "ssh" {
		t.Fatalf("expected ssh command, got %q", spec.Command)
	}
}

func TestBuildProcessSpecDispatchesSSHDynamicProfile(t *testing.T) {
	t.Parallel()

	spec, err := BuildProcessSpec(domain.Profile{
		Name:      "dev-socks",
		Type:      domain.TunnelTypeSSHDynamic,
		LocalPort: 1080,
		SSHDynamic: &domain.SSHDynamic{
			Host: "bastion-prod",
		},
	})
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}

	if spec.Command != "ssh" {
		t.Fatalf("expected ssh command, got %q", spec.Command)
	}
}

func TestBuildProcessSpecRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	_, err := BuildProcessSpec(domain.Profile{
		Name: "mystery",
		Type: domain.TunnelType("custom"),
	})
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
}
