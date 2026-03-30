package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func TestSystemProfileProbeCheckerUsesSSHGForAliasResolution(t *testing.T) {
	checker := newSystemProfileProbeChecker()
	checker.runner = fakeCommandOutputRunner{
		results: map[string]fakeCommandOutputResult{
			"ssh -G bastion-prod": {
				output: "host bastion-prod\nhostname bastion.internal\nuser dev\nport 2222\n",
			},
		},
	}

	result := checker.CheckProfile(testSSHProfile("bastion-prod"), true)
	if len(result.Problems) != 0 {
		t.Fatalf("expected no problems, got %#v", result.Problems)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings when ssh -G resolves alias, got %#v", result.Warnings)
	}
}

func TestSystemProfileProbeCheckerWarnsWhenSSHGInspectionFails(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0o755); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".ssh", "config"), []byte("Host bastion-prod\n  HostName bastion.internal\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	checker := newSystemProfileProbeChecker()
	checker.runner = fakeCommandOutputRunner{
		results: map[string]fakeCommandOutputResult{
			"ssh -G bastion-prod": {
				output: "Bad configuration option: UseKeychain\n",
				err:    errors.New("exit status 255"),
			},
		},
	}

	result := checker.CheckProfile(testSSHProfile("bastion-prod"), true)
	if len(result.Problems) != 0 {
		t.Fatalf("expected no hard problems, got %#v", result.Problems)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], `could not verify SSH host alias "bastion-prod" with ssh -G`) {
		t.Fatalf("expected ssh -G inspection warning, got %#v", result.Warnings)
	}
}

func TestSystemProfileProbeCheckerFallsBackToImportedAliasesAfterSSHGNoop(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0o755); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".ssh", "config"), []byte("Host bastion-prod\n  HostName bastion.internal\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	checker := newSystemProfileProbeChecker()
	checker.runner = fakeCommandOutputRunner{
		results: map[string]fakeCommandOutputResult{
			"ssh -G bastion-prod": {
				output: "host bastion-prod\nhostname bastion-prod\n",
			},
		},
	}

	result := checker.CheckProfile(testSSHProfile("bastion-prod"), true)
	if len(result.Problems) != 0 {
		t.Fatalf("expected no problems, got %#v", result.Problems)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected imported alias fallback to suppress warnings, got %#v", result.Warnings)
	}
}

func TestSystemProfileProbeCheckerWarnsWhenAliasStillMissing(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0o755); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".ssh", "config"), []byte("Host other-host\n  HostName other.internal\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	checker := newSystemProfileProbeChecker()
	checker.runner = fakeCommandOutputRunner{
		results: map[string]fakeCommandOutputResult{
			"ssh -G missing-alias": {
				output: "host missing-alias\nhostname missing-alias\n",
			},
		},
	}

	result := checker.CheckProfile(testSSHProfile("missing-alias"), true)
	if len(result.Problems) != 0 {
		t.Fatalf("expected no problems, got %#v", result.Problems)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], `SSH host alias "missing-alias" was not found`) {
		t.Fatalf("expected missing alias warning, got %#v", result.Warnings)
	}
}

func TestSystemProfileProbeCheckerWarnsWhenConfiguredIdentityFileIsMissing(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	if err := os.MkdirAll(filepath.Join(homeDir, ".ssh"), 0o755); err != nil {
		t.Fatalf("mkdir ssh dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".ssh", "config"), []byte("Host bastion-prod\n  HostName bastion.internal\n  IdentityFile ~/.ssh/work_ed25519\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	checker := newSystemProfileProbeChecker()
	checker.runner = fakeCommandOutputRunner{
		results: map[string]fakeCommandOutputResult{
			"ssh -G bastion-prod": {
				output: "host bastion-prod\nhostname bastion.internal\nidentityfile ~/.ssh/work_ed25519\n",
			},
		},
	}

	result := checker.CheckProfile(testSSHProfile("bastion-prod"), true)
	if len(result.Problems) != 0 {
		t.Fatalf("expected no problems, got %#v", result.Problems)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], `configured SSH identity file "~/.ssh/work_ed25519"`) {
		t.Fatalf("expected missing identity-file warning, got %#v", result.Warnings)
	}
}

func TestSystemProfileProbeCheckerUsesKubectlCurrentContextWhenProfileContextIsEmpty(t *testing.T) {
	kubeconfigPath := filepath.Join(t.TempDir(), "config.kube")
	if err := os.WriteFile(kubeconfigPath, []byte(`
contexts:
  - name: dev-cluster
    context:
      cluster: dev
      user: dev-user
      namespace: backend
`), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}
	t.Setenv("KUBECONFIG", kubeconfigPath)

	checker := newSystemProfileProbeChecker()
	checker.runner = fakeCommandOutputRunner{
		results: map[string]fakeCommandOutputResult{
			"kubectl config current-context": {
				output: "dev-cluster\n",
			},
			"kubectl --request-timeout=5s --context dev-cluster get namespace backend -o name --ignore-not-found": {
				output: "namespace/backend\n",
			},
			"kubectl --request-timeout=5s --context dev-cluster --namespace backend get service api -o name --ignore-not-found": {
				output: "service/api\n",
			},
		},
	}

	result := checker.CheckProfile(domain.Profile{
		Name:      "api-debug",
		Type:      domain.TunnelTypeKubernetesPortForward,
		LocalPort: 18080,
		Kubernetes: &domain.Kubernetes{
			Namespace:    "backend",
			ResourceType: "service",
			Resource:     "api",
			RemotePort:   80,
		},
	}, true)
	if len(result.Problems) != 0 {
		t.Fatalf("expected no problems, got %#v", result.Problems)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", result.Warnings)
	}
}

type fakeCommandOutputRunner struct {
	results map[string]fakeCommandOutputResult
}

type fakeCommandOutputResult struct {
	output string
	err    error
}

func (f fakeCommandOutputRunner) CombinedOutput(_ context.Context, name string, args []string) ([]byte, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	result, exists := f.results[key]
	if !exists {
		return nil, fmt.Errorf("unexpected command %q", key)
	}
	return []byte(result.output), result.err
}

func testSSHProfile(host string) domain.Profile {
	return domain.Profile{
		Name:      "prod-db",
		Type:      domain.TunnelTypeSSHLocal,
		LocalPort: 15432,
		SSH: &domain.SSHLocal{
			Host:       host,
			RemoteHost: "db.internal",
			RemotePort: 5432,
		},
	}
}
