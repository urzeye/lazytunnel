package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const CurrentConfigVersion = 1

type TunnelType string

const (
	TunnelTypeSSHLocal              TunnelType = "ssh_local"
	TunnelTypeKubernetesPortForward TunnelType = "kubernetes_port_forward"
)

var supportedTunnelTypes = []TunnelType{
	TunnelTypeSSHLocal,
	TunnelTypeKubernetesPortForward,
}

type Config struct {
	Version  int       `yaml:"version"`
	Profiles []Profile `yaml:"profiles"`
	Stacks   []Stack   `yaml:"stacks"`
}

type Profile struct {
	Name        string        `yaml:"name"`
	Description string        `yaml:"description,omitempty"`
	Type        TunnelType    `yaml:"type"`
	LocalPort   int           `yaml:"local_port"`
	Labels      []string      `yaml:"labels,omitempty"`
	Restart     RestartPolicy `yaml:"restart"`
	SSH         *SSHLocal     `yaml:"ssh,omitempty"`
	Kubernetes  *Kubernetes   `yaml:"kubernetes,omitempty"`
}

type SSHLocal struct {
	Host       string `yaml:"host"`
	RemoteHost string `yaml:"remote_host"`
	RemotePort int    `yaml:"remote_port"`
}

type Kubernetes struct {
	Context      string `yaml:"context,omitempty"`
	Namespace    string `yaml:"namespace,omitempty"`
	ResourceType string `yaml:"resource_type"`
	Resource     string `yaml:"resource"`
	RemotePort   int    `yaml:"remote_port"`
}

type RestartPolicy struct {
	Enabled        bool   `yaml:"enabled"`
	MaxRetries     int    `yaml:"max_retries,omitempty"`
	InitialBackoff string `yaml:"initial_backoff,omitempty"`
	MaxBackoff     string `yaml:"max_backoff,omitempty"`
}

type Stack struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Labels      []string `yaml:"labels,omitempty"`
	Profiles    []string `yaml:"profiles"`
}

func DefaultConfig() Config {
	return Config{Version: CurrentConfigVersion}
}

func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = CurrentConfigVersion
	}
}

func (c Config) Validate() error {
	var errs []error

	if c.Version != CurrentConfigVersion {
		errs = append(errs, fmt.Errorf("unsupported config version %d", c.Version))
	}

	profileNames := make(map[string]struct{}, len(c.Profiles))
	for idx, profile := range c.Profiles {
		if err := profile.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("profile[%d] %q: %w", idx, profile.Name, err))
		}

		if _, exists := profileNames[profile.Name]; exists {
			errs = append(errs, fmt.Errorf("duplicate profile name %q", profile.Name))
		}

		profileNames[profile.Name] = struct{}{}
	}

	stackNames := make(map[string]struct{}, len(c.Stacks))
	for idx, stack := range c.Stacks {
		if err := stack.Validate(profileNames); err != nil {
			errs = append(errs, fmt.Errorf("stack[%d] %q: %w", idx, stack.Name, err))
		}

		if _, exists := stackNames[stack.Name]; exists {
			errs = append(errs, fmt.Errorf("duplicate stack name %q", stack.Name))
		}

		stackNames[stack.Name] = struct{}{}
	}

	return errors.Join(errs...)
}

func (p Profile) Validate() error {
	var errs []error

	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}

	if !slices.Contains(supportedTunnelTypes, p.Type) {
		errs = append(errs, fmt.Errorf("unsupported tunnel type %q", p.Type))
	}

	if err := validatePort("local_port", p.LocalPort); err != nil {
		errs = append(errs, err)
	}

	if err := p.Restart.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("restart: %w", err))
	}

	switch p.Type {
	case TunnelTypeSSHLocal:
		if p.SSH == nil {
			errs = append(errs, errors.New("ssh settings are required"))
			break
		}

		if err := p.SSH.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("ssh: %w", err))
		}

	case TunnelTypeKubernetesPortForward:
		if p.Kubernetes == nil {
			errs = append(errs, errors.New("kubernetes settings are required"))
			break
		}

		if err := p.Kubernetes.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("kubernetes: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (s SSHLocal) Validate() error {
	var errs []error

	if strings.TrimSpace(s.Host) == "" {
		errs = append(errs, errors.New("host is required"))
	}

	if strings.TrimSpace(s.RemoteHost) == "" {
		errs = append(errs, errors.New("remote_host is required"))
	}

	if err := validatePort("remote_port", s.RemotePort); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (k Kubernetes) Validate() error {
	var errs []error

	switch k.ResourceType {
	case "pod", "service", "deployment":
	default:
		errs = append(errs, fmt.Errorf("resource_type must be one of pod, service, deployment"))
	}

	if strings.TrimSpace(k.Resource) == "" {
		errs = append(errs, errors.New("resource is required"))
	}

	if err := validatePort("remote_port", k.RemotePort); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (r RestartPolicy) Validate() error {
	var errs []error

	if r.MaxRetries < 0 {
		errs = append(errs, errors.New("max_retries must be greater than or equal to 0"))
	}

	if r.InitialBackoff != "" {
		if _, err := time.ParseDuration(r.InitialBackoff); err != nil {
			errs = append(errs, fmt.Errorf("invalid initial_backoff: %w", err))
		}
	}

	if r.MaxBackoff != "" {
		if _, err := time.ParseDuration(r.MaxBackoff); err != nil {
			errs = append(errs, fmt.Errorf("invalid max_backoff: %w", err))
		}
	}

	return errors.Join(errs...)
}

func (s Stack) Validate(profileNames map[string]struct{}) error {
	var errs []error

	if strings.TrimSpace(s.Name) == "" {
		errs = append(errs, errors.New("name is required"))
	}

	if len(s.Profiles) == 0 {
		errs = append(errs, errors.New("profiles must include at least one profile name"))
	}

	for _, profileName := range s.Profiles {
		if _, exists := profileNames[profileName]; !exists {
			errs = append(errs, fmt.Errorf("references unknown profile %q", profileName))
		}
	}

	return errors.Join(errs...)
}

func validatePort(field string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", field)
	}

	return nil
}
