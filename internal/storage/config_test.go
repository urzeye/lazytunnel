package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func TestLoadConfigReturnsDefaultWhenFileDoesNotExist(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("expected missing config to return defaults, got %v", err)
	}

	if cfg.Version != domain.CurrentConfigVersion {
		t.Fatalf("expected default version %d, got %d", domain.CurrentConfigVersion, cfg.Version)
	}
	if cfg.Language != domain.LanguageEnglish {
		t.Fatalf("expected default language %q, got %q", domain.LanguageEnglish, cfg.Language)
	}
}

func TestLoadConfigParsesExampleConfig(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig(filepath.Join("..", "..", "config.example.yaml"))
	if err != nil {
		t.Fatalf("expected example config to load, got %v", err)
	}

	if got, want := len(cfg.Profiles), 2; got != want {
		t.Fatalf("expected %d profiles, got %d", want, got)
	}

	if got, want := len(cfg.Stacks), 1; got != want {
		t.Fatalf("expected %d stack, got %d", want, got)
	}
	if cfg.Language != domain.LanguageEnglish {
		t.Fatalf("expected example language %q, got %q", domain.LanguageEnglish, cfg.Language)
	}
}

func TestSaveConfigRoundTrips(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	want := SampleConfig()

	if err := SaveConfig(path, want); err != nil {
		t.Fatalf("save config: %v", err)
	}

	got, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(got.Profiles) != len(want.Profiles) {
		t.Fatalf("expected %d profiles, got %d", len(want.Profiles), len(got.Profiles))
	}
	if len(got.Stacks) != len(want.Stacks) {
		t.Fatalf("expected %d stacks, got %d", len(want.Stacks), len(got.Stacks))
	}
}

func TestSaveConfigCreatesParentDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	if err := SaveConfig(path, domain.DefaultConfig()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config to exist: %v", err)
	}
}
