package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func DefaultConfigPath() string {
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		return filepath.Join(xdgConfigHome, "lazytunnel", "config.yaml")
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "lazytunnel", "config.yaml")
	}

	return filepath.Join(homeDir, ".config", "lazytunnel", "config.yaml")
}

func LoadConfig(path string) (domain.Config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.DefaultConfig(), nil
		}

		return domain.Config{}, fmt.Errorf("read file: %w", err)
	}

	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return domain.Config{}, fmt.Errorf("decode yaml: %w", err)
	}

	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return domain.Config{}, err
	}

	return cfg, nil
}
