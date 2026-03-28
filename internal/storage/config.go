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

func SaveConfig(path string, cfg domain.Config) error {
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close encoder: %w", err)
	}

	return nil
}

func SampleConfig() domain.Config {
	return domain.Config{
		Version:  domain.CurrentConfigVersion,
		Language: domain.LanguageEnglish,
		Profiles: []domain.Profile{
			{
				Name:        "prod-db",
				Description: "Access the production database through an SSH bastion.",
				Type:        domain.TunnelTypeSSHLocal,
				LocalPort:   5432,
				Labels:      []string{"prod", "db"},
				Restart: domain.RestartPolicy{
					Enabled:        true,
					MaxRetries:     0,
					InitialBackoff: "2s",
					MaxBackoff:     "30s",
				},
				SSH: &domain.SSHLocal{
					Host:       "bastion-prod",
					RemoteHost: "db.internal",
					RemotePort: 5432,
				},
			},
			{
				Name:        "api-debug",
				Description: "Forward the API service from the dev cluster.",
				Type:        domain.TunnelTypeKubernetesPortForward,
				LocalPort:   8080,
				Labels:      []string{"dev", "api"},
				Restart: domain.RestartPolicy{
					Enabled:        true,
					MaxRetries:     0,
					InitialBackoff: "2s",
					MaxBackoff:     "30s",
				},
				Kubernetes: &domain.Kubernetes{
					Context:      "dev-cluster",
					Namespace:    "backend",
					ResourceType: "service",
					Resource:     "api",
					RemotePort:   80,
				},
			},
		},
		Stacks: []domain.Stack{
			{
				Name:        "backend-dev",
				Description: "Daily backend development stack.",
				Profiles:    []string{"prod-db", "api-debug"},
			},
		},
	}
}
