package storage

import (
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
}
