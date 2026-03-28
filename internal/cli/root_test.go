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
