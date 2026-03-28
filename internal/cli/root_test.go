package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
	"github.com/urzeye/lazytunnel/internal/storage"
)

func TestInitCommandCreatesSampleConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	output := executeCommand(t,
		"--config", configPath,
		"init", "--sample",
	)

	if !strings.Contains(output, "initialized sample config") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got == 0 {
		t.Fatal("expected sample config to include profiles")
	}
}

func TestValidateCommandReportsCounts(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"validate",
	)

	if !strings.Contains(output, "config is valid: 2 profiles, 1 stacks") {
		t.Fatalf("unexpected output: %q", output)
	}
}

func TestProfileAddSSHLocalPersistsProfile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, domain.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "add", "ssh-local",
		"--name", "prod-db",
		"--host", "bastion-prod",
		"--remote-host", "db.internal",
		"--remote-port", "5432",
		"--local-port", "5432",
		"--label", "prod",
		"--label", "db",
	)

	if !strings.Contains(output, "added profile prod-db") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 1 {
		t.Fatalf("expected 1 profile, got %d", got)
	}
	if got := cfg.Profiles[0].Type; got != "ssh_local" {
		t.Fatalf("expected ssh_local profile, got %q", got)
	}
}

func TestStackAddPersistsStack(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, domain.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	executeCommand(t,
		"--config", configPath,
		"profile", "add", "ssh-local",
		"--name", "prod-db",
		"--host", "bastion-prod",
		"--remote-host", "db.internal",
		"--remote-port", "5432",
		"--local-port", "5432",
	)

	output := executeCommand(t,
		"--config", configPath,
		"stack", "add",
		"--name", "backend-dev",
		"--profile", "prod-db",
	)

	if !strings.Contains(output, "added stack backend-dev") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Stacks); got != 1 {
		t.Fatalf("expected 1 stack, got %d", got)
	}
}

func TestVersionCommandReportsBuildInfo(t *testing.T) {
	t.Parallel()

	output := executeCommand(t, "version")

	for _, expected := range []string{
		"version: dev",
		"commit: none",
		"built: unknown",
		"os/arch:",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestVersionCommandShort(t *testing.T) {
	t.Parallel()

	output := executeCommand(t, "version", "--short")
	if strings.TrimSpace(output) != "dev" {
		t.Fatalf("expected short version output to be dev, got %q", output)
	}
}

func TestProfileRemoveDeletesExistingProfile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := domain.DefaultConfig()
	cfg.Profiles = []domain.Profile{
		{
			Name:      "api-debug",
			Type:      domain.TunnelTypeKubernetesPortForward,
			LocalPort: 8080,
			Kubernetes: &domain.Kubernetes{
				ResourceType: "service",
				Resource:     "api",
				RemotePort:   80,
			},
		},
	}
	if err := storage.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "remove", "api-debug",
	)

	if !strings.Contains(output, "removed profile api-debug") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 0 {
		t.Fatalf("expected 0 profiles, got %d", got)
	}
}

func TestProfileRemoveRejectsReferencedProfileWithoutFlag(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output, err := executeCommandErr(t,
		"--config", configPath,
		"profile", "remove", "prod-db",
	)
	if err == nil {
		t.Fatal("expected remove command to fail")
	}

	if !strings.Contains(err.Error(), "--remove-from-stacks") {
		t.Fatalf("expected guidance about --remove-from-stacks, got err=%q output=%q", err, output)
	}
}

func TestProfileRemoveCanPruneStackReferences(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "remove", "prod-db",
		"--remove-from-stacks",
	)

	if !strings.Contains(output, "pruned 1 stack references") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 1 {
		t.Fatalf("expected 1 profile, got %d", got)
	}
	if got := len(cfg.Stacks); got != 1 {
		t.Fatalf("expected 1 stack, got %d", got)
	}
	if want := "api-debug"; cfg.Stacks[0].Profiles[0] != want {
		t.Fatalf("expected remaining stack member %q, got %q", want, cfg.Stacks[0].Profiles[0])
	}
}

func TestStackRemoveDeletesExistingStack(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"stack", "remove", "backend-dev",
	)

	if !strings.Contains(output, "removed stack backend-dev") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Stacks); got != 0 {
		t.Fatalf("expected 0 stacks, got %d", got)
	}
}

func executeCommand(t *testing.T, args ...string) string {
	t.Helper()

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute command %v: %v\noutput:\n%s", args, err, stdout.String())
	}

	return stdout.String()
}

func executeCommandErr(t *testing.T, args ...string) (string, error) {
	t.Helper()

	cmd := NewRootCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs(args)

	return stdout.String(), cmd.Execute()
}
