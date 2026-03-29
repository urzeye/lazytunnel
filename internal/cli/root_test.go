package cli

import (
	"bytes"
	"os"
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

func TestProfileAddSSHRemotePersistsProfile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, domain.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "add", "ssh-remote",
		"--name", "public-api",
		"--host", "bastion-prod",
		"--bind-address", "0.0.0.0",
		"--bind-port", "9000",
		"--target-host", "127.0.0.1",
		"--target-port", "8080",
	)

	if !strings.Contains(output, "added profile public-api") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 1 {
		t.Fatalf("expected 1 profile, got %d", got)
	}
	if got := cfg.Profiles[0].Type; got != domain.TunnelTypeSSHRemote {
		t.Fatalf("expected ssh_remote profile, got %q", got)
	}
	if cfg.Profiles[0].SSHRemote == nil || cfg.Profiles[0].SSHRemote.BindPort != 9000 || cfg.Profiles[0].SSHRemote.TargetPort != 8080 {
		t.Fatalf("unexpected ssh remote settings: %#v", cfg.Profiles[0].SSHRemote)
	}
}

func TestProfileAddSSHDynamicPersistsProfile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, domain.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "add", "ssh-dynamic",
		"--name", "dev-socks",
		"--host", "bastion-prod",
		"--bind-address", "127.0.0.1",
		"--local-port", "1080",
	)

	if !strings.Contains(output, "added profile dev-socks") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 1 {
		t.Fatalf("expected 1 profile, got %d", got)
	}
	if got := cfg.Profiles[0].Type; got != domain.TunnelTypeSSHDynamic {
		t.Fatalf("expected ssh_dynamic profile, got %q", got)
	}
	if cfg.Profiles[0].SSHDynamic == nil || cfg.Profiles[0].SSHDynamic.BindAddress != "127.0.0.1" || cfg.Profiles[0].LocalPort != 1080 {
		t.Fatalf("unexpected ssh dynamic settings: %#v", cfg.Profiles[0].SSHDynamic)
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

func TestProfileCloneCopiesProfileWithOverrides(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "clone", "prod-db",
		"--name", "staging-db",
		"--local-port", "15432",
		"--description", "Staging database tunnel",
		"--clear-labels",
		"--label", "staging",
		"--label", "db",
	)

	if !strings.Contains(output, "cloned profile staging-db from prod-db") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 3 {
		t.Fatalf("expected 3 profiles, got %d", got)
	}

	var cloned domain.Profile
	for _, profile := range cfg.Profiles {
		if profile.Name == "staging-db" {
			cloned = profile
			break
		}
	}

	if cloned.Name == "" {
		t.Fatal("expected cloned profile to exist")
	}
	if cloned.LocalPort != 15432 {
		t.Fatalf("expected cloned local port 15432, got %d", cloned.LocalPort)
	}
	if cloned.Description != "Staging database tunnel" {
		t.Fatalf("unexpected description: %q", cloned.Description)
	}
	if got := strings.Join(cloned.Labels, ","); got != "staging,db" {
		t.Fatalf("unexpected labels: %q", got)
	}
	if cloned.SSH == nil || cloned.SSH.Host != "bastion-prod" {
		t.Fatalf("expected SSH settings to be copied, got %#v", cloned.SSH)
	}
}

func TestProfileEditRenamesProfileAndUpdatesStackReferences(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "edit", "prod-db",
		"--name", "staging-db",
		"--local-port", "15432",
		"--description", "Staging database tunnel",
	)

	if !strings.Contains(output, "updated profile staging-db (renamed from prod-db)") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if _, exists := findProfile(cfg.Profiles, "prod-db"); exists {
		t.Fatal("expected old profile name to be removed")
	}

	edited, exists := findProfile(cfg.Profiles, "staging-db")
	if !exists {
		t.Fatal("expected renamed profile to exist")
	}
	if edited.LocalPort != 15432 {
		t.Fatalf("expected edited local port 15432, got %d", edited.LocalPort)
	}
	if got := strings.Join(cfg.Stacks[0].Profiles, ","); got != "staging-db,api-debug" {
		t.Fatalf("expected stack references to be updated, got %q", got)
	}
}

func TestProfileEditUpdatesKubernetesFields(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "edit", "api-debug",
		"--context", "staging-cluster",
		"--namespace", "payments",
		"--resource-type", "deployment",
		"--resource", "api-v2",
		"--remote-port", "8081",
		"--clear-labels",
		"--label", "staging",
	)

	if !strings.Contains(output, "updated profile api-debug") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	edited, exists := findProfile(cfg.Profiles, "api-debug")
	if !exists {
		t.Fatal("expected edited profile to exist")
	}
	if edited.Kubernetes == nil {
		t.Fatal("expected kubernetes settings to exist")
	}
	if edited.Kubernetes.Context != "staging-cluster" || edited.Kubernetes.Namespace != "payments" {
		t.Fatalf("unexpected kubernetes location: %#v", edited.Kubernetes)
	}
	if edited.Kubernetes.ResourceType != "deployment" || edited.Kubernetes.Resource != "api-v2" || edited.Kubernetes.RemotePort != 8081 {
		t.Fatalf("unexpected kubernetes target: %#v", edited.Kubernetes)
	}
	if got := strings.Join(edited.Labels, ","); got != "staging" {
		t.Fatalf("unexpected labels: %q", got)
	}
}

func TestProfileEditUpdatesSSHRemoteFields(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := domain.DefaultConfig()
	cfg.Profiles = []domain.Profile{
		{
			Name:      "public-api",
			Type:      domain.TunnelTypeSSHRemote,
			LocalPort: 9000,
			SSHRemote: &domain.SSHRemote{
				Host:       "bastion-prod",
				BindPort:   9000,
				TargetHost: "127.0.0.1",
				TargetPort: 8080,
			},
		},
	}
	if err := storage.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "edit", "public-api",
		"--bind-address", "0.0.0.0",
		"--bind-port", "9443",
		"--target-host", "127.0.0.1",
		"--target-port", "8443",
	)

	if !strings.Contains(output, "updated profile public-api") {
		t.Fatalf("unexpected output: %q", output)
	}

	persisted, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	edited, exists := findProfile(persisted.Profiles, "public-api")
	if !exists {
		t.Fatal("expected edited profile to exist")
	}
	if edited.SSHRemote == nil {
		t.Fatal("expected ssh remote settings to exist")
	}
	if edited.SSHRemote.BindAddress != "0.0.0.0" || edited.SSHRemote.BindPort != 9443 {
		t.Fatalf("unexpected bind settings: %#v", edited.SSHRemote)
	}
	if edited.SSHRemote.TargetHost != "127.0.0.1" || edited.SSHRemote.TargetPort != 8443 {
		t.Fatalf("unexpected target settings: %#v", edited.SSHRemote)
	}
}

func TestProfileEditUpdatesSSHDynamicFields(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := domain.DefaultConfig()
	cfg.Profiles = []domain.Profile{
		{
			Name:      "dev-socks",
			Type:      domain.TunnelTypeSSHDynamic,
			LocalPort: 1080,
			SSHDynamic: &domain.SSHDynamic{
				Host: "bastion-prod",
			},
		},
	}
	if err := storage.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "edit", "dev-socks",
		"--host", "bastion-staging",
		"--bind-address", "127.0.0.1",
		"--local-port", "2080",
	)

	if !strings.Contains(output, "updated profile dev-socks") {
		t.Fatalf("unexpected output: %q", output)
	}

	persisted, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	edited, exists := findProfile(persisted.Profiles, "dev-socks")
	if !exists {
		t.Fatal("expected edited profile to exist")
	}
	if edited.SSHDynamic == nil {
		t.Fatal("expected ssh dynamic settings to exist")
	}
	if edited.SSHDynamic.Host != "bastion-staging" || edited.SSHDynamic.BindAddress != "127.0.0.1" {
		t.Fatalf("unexpected SSH dynamic endpoint: %#v", edited.SSHDynamic)
	}
	if edited.LocalPort != 2080 {
		t.Fatalf("expected local port 2080, got %d", edited.LocalPort)
	}
}

func TestProfileEditRejectsWrongFamilyFlags(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	_, err := executeCommandErr(t,
		"--config", configPath,
		"profile", "edit", "prod-db",
		"--context", "dev-cluster",
	)
	if err == nil {
		t.Fatal("expected edit command to fail")
	}

	if !strings.Contains(err.Error(), "kubernetes-specific flags") {
		t.Fatalf("unexpected error: %v", err)
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

func TestStackCloneCopiesMembersWithOverrides(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"stack", "clone", "backend-dev",
		"--name", "backend-staging",
		"--description", "Staging backend stack",
		"--profile", "api-debug",
		"--label", "staging",
	)

	if !strings.Contains(output, "cloned stack backend-staging from backend-dev") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Stacks); got != 2 {
		t.Fatalf("expected 2 stacks, got %d", got)
	}

	var cloned domain.Stack
	for _, stack := range cfg.Stacks {
		if stack.Name == "backend-staging" {
			cloned = stack
			break
		}
	}

	if cloned.Name == "" {
		t.Fatal("expected cloned stack to exist")
	}
	if cloned.Description != "Staging backend stack" {
		t.Fatalf("unexpected description: %q", cloned.Description)
	}
	if got := strings.Join(cloned.Profiles, ","); got != "api-debug" {
		t.Fatalf("unexpected profiles: %q", got)
	}
	if got := strings.Join(cloned.Labels, ","); got != "staging" {
		t.Fatalf("unexpected labels: %q", got)
	}
}

func TestStackEditRenamesAndReplacesMembers(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, storage.SampleConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"stack", "edit", "backend-dev",
		"--name", "backend-staging",
		"--description", "Staging backend stack",
		"--profile", "api-debug",
		"--clear-labels",
		"--label", "staging",
	)

	if !strings.Contains(output, "updated stack backend-staging (renamed from backend-dev)") {
		t.Fatalf("unexpected output: %q", output)
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if _, exists := findStack(cfg.Stacks, "backend-dev"); exists {
		t.Fatal("expected old stack name to be removed")
	}

	edited, exists := findStack(cfg.Stacks, "backend-staging")
	if !exists {
		t.Fatal("expected edited stack to exist")
	}
	if edited.Description != "Staging backend stack" {
		t.Fatalf("unexpected description: %q", edited.Description)
	}
	if got := strings.Join(edited.Profiles, ","); got != "api-debug" {
		t.Fatalf("unexpected profiles: %q", got)
	}
	if got := strings.Join(edited.Labels, ","); got != "staging" {
		t.Fatalf("unexpected labels: %q", got)
	}
}

func TestProfileImportSSHConfigCreatesDraftProfiles(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, domain.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	sshConfigPath := filepath.Join(t.TempDir(), "ssh_config")
	sshConfig := `
Host *
  User shared-user

Host bastion-prod bastion-alt *.ignored
  HostName bastion.internal
  User deploy
  Port 2222

Host jump-dev
  HostName jump.internal
`
	if err := os.WriteFile(sshConfigPath, []byte(strings.TrimSpace(sshConfig)+"\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "import", "ssh-config",
		"--path", sshConfigPath,
	)

	if !strings.Contains(output, "3 created") {
		t.Fatalf("unexpected output: %q", output)
	}
	for _, name := range []string{"bastion-prod", "bastion-alt", "jump-dev"} {
		if !strings.Contains(output, name) {
			t.Fatalf("expected imported name %q in output, got %q", name, output)
		}
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := len(cfg.Profiles); got != 3 {
		t.Fatalf("expected 3 imported profiles, got %d", got)
	}

	imported, exists := findProfile(cfg.Profiles, "bastion-prod")
	if !exists {
		t.Fatal("expected imported bastion-prod profile")
	}
	if imported.Type != domain.TunnelTypeSSHLocal {
		t.Fatalf("expected ssh_local type, got %q", imported.Type)
	}
	if imported.SSH == nil || imported.SSH.Host != "bastion-prod" {
		t.Fatalf("unexpected SSH import profile: %#v", imported.SSH)
	}
	if imported.SSH.RemoteHost != "127.0.0.1" || imported.SSH.RemotePort != 80 {
		t.Fatalf("expected placeholder target on imported profile, got %#v", imported.SSH)
	}
	if !strings.Contains(imported.Description, "HostName bastion.internal.") {
		t.Fatalf("expected HostName in description, got %q", imported.Description)
	}
	if !strings.Contains(imported.Description, "SSH port 2222.") {
		t.Fatalf("expected SSH port in description, got %q", imported.Description)
	}
	if got := strings.Join(imported.Labels, ","); got != "draft,imported,ssh-config" {
		t.Fatalf("unexpected labels: %q", got)
	}
}

func TestProfileImportSSHConfigSkipsExistingWithoutOverwrite(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := domain.DefaultConfig()
	cfg.Profiles = []domain.Profile{
		{
			Name:      "jump-dev",
			Type:      domain.TunnelTypeSSHLocal,
			LocalPort: 15432,
			SSH: &domain.SSHLocal{
				Host:       "jump-dev",
				RemoteHost: "db.internal",
				RemotePort: 5432,
			},
		},
	}
	if err := storage.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	sshConfigPath := filepath.Join(t.TempDir(), "ssh_config")
	sshConfig := `
Host jump-dev
  HostName jump.internal
`
	if err := os.WriteFile(sshConfigPath, []byte(strings.TrimSpace(sshConfig)+"\n"), 0o644); err != nil {
		t.Fatalf("write ssh config: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "import", "ssh-config",
		"--path", sshConfigPath,
	)

	if !strings.Contains(output, "0 created, 0 updated, 1 skipped") {
		t.Fatalf("unexpected output: %q", output)
	}

	persisted, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	existing, exists := findProfile(persisted.Profiles, "jump-dev")
	if !exists {
		t.Fatal("expected existing profile to remain")
	}
	if existing.SSH == nil || existing.SSH.RemoteHost != "db.internal" || existing.SSH.RemotePort != 5432 {
		t.Fatalf("expected existing profile to remain unchanged, got %#v", existing.SSH)
	}
}

func TestProfileImportKubeContextsCreatesDraftProfiles(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := storage.SaveConfig(configPath, domain.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	kubeconfigPath := filepath.Join(t.TempDir(), "config")
	kubeconfig := `
apiVersion: v1
kind: Config
current-context: dev-cluster
contexts:
  - name: dev-cluster
    context:
      cluster: dev
      user: dev-user
      namespace: backend
  - name: prod-cluster
    context:
      cluster: prod
      user: prod-user
`
	if err := os.WriteFile(kubeconfigPath, []byte(strings.TrimSpace(kubeconfig)+"\n"), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "import", "kube-contexts",
		"--kubeconfig", kubeconfigPath,
	)

	if !strings.Contains(output, "2 created") {
		t.Fatalf("unexpected output: %q", output)
	}
	for _, name := range []string{"dev-cluster", "prod-cluster"} {
		if !strings.Contains(output, name) {
			t.Fatalf("expected imported context %q in output, got %q", name, output)
		}
	}

	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	imported, exists := findProfile(cfg.Profiles, "dev-cluster")
	if !exists {
		t.Fatal("expected imported dev-cluster profile")
	}
	if imported.Type != domain.TunnelTypeKubernetesPortForward {
		t.Fatalf("expected kubernetes type, got %q", imported.Type)
	}
	if imported.Kubernetes == nil {
		t.Fatal("expected kubernetes config to be present")
	}
	if imported.Kubernetes.Context != "dev-cluster" {
		t.Fatalf("unexpected imported context: %#v", imported.Kubernetes)
	}
	if imported.Kubernetes.Namespace != "backend" {
		t.Fatalf("expected imported namespace backend, got %q", imported.Kubernetes.Namespace)
	}
	if imported.Kubernetes.ResourceType != "service" || imported.Kubernetes.Resource != "change-me" || imported.Kubernetes.RemotePort != 80 {
		t.Fatalf("expected placeholder resource target, got %#v", imported.Kubernetes)
	}
	if !strings.Contains(imported.Description, "Cluster dev.") {
		t.Fatalf("expected cluster details in description, got %q", imported.Description)
	}
	if !strings.Contains(imported.Description, "Namespace backend.") {
		t.Fatalf("expected namespace details in description, got %q", imported.Description)
	}
	if got := strings.Join(imported.Labels, ","); got != "draft,imported,kube-context,current-context" {
		t.Fatalf("unexpected labels: %q", got)
	}
}

func TestProfileImportKubeContextsSkipsExistingWithoutOverwrite(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := domain.DefaultConfig()
	cfg.Profiles = []domain.Profile{
		{
			Name:      "dev-cluster",
			Type:      domain.TunnelTypeKubernetesPortForward,
			LocalPort: 18080,
			Kubernetes: &domain.Kubernetes{
				Context:      "dev-cluster",
				Namespace:    "custom",
				ResourceType: "service",
				Resource:     "api",
				RemotePort:   8080,
			},
		},
	}
	if err := storage.SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	kubeconfigPath := filepath.Join(t.TempDir(), "config")
	kubeconfig := `
apiVersion: v1
kind: Config
contexts:
  - name: dev-cluster
    context:
      cluster: dev
`
	if err := os.WriteFile(kubeconfigPath, []byte(strings.TrimSpace(kubeconfig)+"\n"), 0o644); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	output := executeCommand(t,
		"--config", configPath,
		"profile", "import", "kube-contexts",
		"--kubeconfig", kubeconfigPath,
	)

	if !strings.Contains(output, "0 created, 0 updated, 1 skipped") {
		t.Fatalf("unexpected output: %q", output)
	}

	persisted, err := storage.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	existing, exists := findProfile(persisted.Profiles, "dev-cluster")
	if !exists {
		t.Fatal("expected existing profile to remain")
	}
	if existing.Kubernetes == nil || existing.Kubernetes.Namespace != "custom" || existing.Kubernetes.Resource != "api" || existing.Kubernetes.RemotePort != 8080 {
		t.Fatalf("expected existing kube profile to remain unchanged, got %#v", existing.Kubernetes)
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
