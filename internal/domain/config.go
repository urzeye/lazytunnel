package domain

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const CurrentConfigVersion = 1

type Language string

const (
	LanguageEnglish           Language = "en"
	LanguageSimplifiedChinese Language = "zh-CN"
)

var supportedLanguages = []Language{
	LanguageEnglish,
	LanguageSimplifiedChinese,
}

type TunnelType string

const (
	TunnelTypeSSHLocal              TunnelType = "ssh_local"
	TunnelTypeSSHRemote             TunnelType = "ssh_remote"
	TunnelTypeSSHDynamic            TunnelType = "ssh_dynamic"
	TunnelTypeKubernetesPortForward TunnelType = "kubernetes_port_forward"
)

var supportedTunnelTypes = []TunnelType{
	TunnelTypeSSHLocal,
	TunnelTypeSSHRemote,
	TunnelTypeSSHDynamic,
	TunnelTypeKubernetesPortForward,
}

type Config struct {
	Version  int       `yaml:"version"`
	Language Language  `yaml:"language"`
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
	SSHRemote   *SSHRemote    `yaml:"ssh_remote,omitempty"`
	SSHDynamic  *SSHDynamic   `yaml:"ssh_dynamic,omitempty"`
	Kubernetes  *Kubernetes   `yaml:"kubernetes,omitempty"`
}

type SSHLocal struct {
	Host       string `yaml:"host"`
	RemoteHost string `yaml:"remote_host"`
	RemotePort int    `yaml:"remote_port"`
}

type SSHRemote struct {
	Host        string `yaml:"host"`
	BindAddress string `yaml:"bind_address,omitempty"`
	BindPort    int    `yaml:"bind_port"`
	TargetHost  string `yaml:"target_host"`
	TargetPort  int    `yaml:"target_port"`
}

type SSHDynamic struct {
	Host        string `yaml:"host"`
	BindAddress string `yaml:"bind_address,omitempty"`
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
	return Config{
		Version:  CurrentConfigVersion,
		Language: LanguageEnglish,
	}
}

func (c *Config) SetProfile(profile Profile) (created bool) {
	for idx := range c.Profiles {
		if c.Profiles[idx].Name == profile.Name {
			c.Profiles[idx] = profile
			return false
		}
	}

	c.Profiles = append(c.Profiles, profile)
	return true
}

func (c *Config) SetStack(stack Stack) (created bool) {
	for idx := range c.Stacks {
		if c.Stacks[idx].Name == stack.Name {
			c.Stacks[idx] = stack
			return false
		}
	}

	c.Stacks = append(c.Stacks, stack)
	return true
}

func (c *Config) RemoveProfile(name string) bool {
	for idx := range c.Profiles {
		if c.Profiles[idx].Name != name {
			continue
		}

		c.Profiles = append(c.Profiles[:idx], c.Profiles[idx+1:]...)
		return true
	}

	return false
}

func (c *Config) RemoveStack(name string) bool {
	for idx := range c.Stacks {
		if c.Stacks[idx].Name != name {
			continue
		}

		c.Stacks = append(c.Stacks[:idx], c.Stacks[idx+1:]...)
		return true
	}

	return false
}

func (c *Config) StacksReferencingProfile(name string) []string {
	names := make([]string, 0)
	for _, stack := range c.Stacks {
		for _, profileName := range stack.Profiles {
			if profileName != name {
				continue
			}

			names = append(names, stack.Name)
			break
		}
	}

	return names
}

func (c *Config) RemoveProfileFromStacks(name string) (updatedStacks, removedStacks int) {
	filtered := c.Stacks[:0]
	for _, stack := range c.Stacks {
		profiles := stack.Profiles[:0]
		removed := false
		for _, profileName := range stack.Profiles {
			if profileName == name {
				removed = true
				continue
			}
			profiles = append(profiles, profileName)
		}

		if removed {
			updatedStacks++
		}

		stack.Profiles = profiles
		if len(stack.Profiles) == 0 {
			if removed {
				removedStacks++
			}
			continue
		}

		filtered = append(filtered, stack)
	}

	c.Stacks = filtered
	return updatedStacks, removedStacks
}

func (c *Config) RenameProfileInStacks(oldName, newName string) int {
	if oldName == newName {
		return 0
	}

	updated := 0
	for stackIdx := range c.Stacks {
		changed := false
		for profileIdx := range c.Stacks[stackIdx].Profiles {
			if c.Stacks[stackIdx].Profiles[profileIdx] != oldName {
				continue
			}

			c.Stacks[stackIdx].Profiles[profileIdx] = newName
			changed = true
		}

		if changed {
			updated++
		}
	}

	return updated
}

func (c *Config) Normalize() {
	if c.Version == 0 {
		c.Version = CurrentConfigVersion
	}
	if c.Language == "" {
		c.Language = LanguageEnglish
	}
}

func (c Config) Validate() error {
	c.Normalize()

	var errs []error

	if c.Version != CurrentConfigVersion {
		errs = append(errs, fmt.Errorf("unsupported config version %d", c.Version))
	}
	if !slices.Contains(supportedLanguages, c.Language) {
		errs = append(errs, fmt.Errorf("unsupported language %q", c.Language))
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

	if err := p.Restart.Validate(); err != nil {
		errs = append(errs, fmt.Errorf("restart: %w", err))
	}

	switch p.Type {
	case TunnelTypeSSHLocal:
		if err := validatePort("local_port", p.LocalPort); err != nil {
			errs = append(errs, err)
		}

		if p.SSH == nil {
			errs = append(errs, errors.New("ssh settings are required"))
			break
		}

		if err := p.SSH.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("ssh: %w", err))
		}

	case TunnelTypeSSHRemote:
		if p.SSHRemote == nil {
			errs = append(errs, errors.New("ssh_remote settings are required"))
			break
		}

		if err := p.SSHRemote.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("ssh_remote: %w", err))
		}

	case TunnelTypeSSHDynamic:
		if err := validatePort("local_port", p.LocalPort); err != nil {
			errs = append(errs, err)
		}

		if p.SSHDynamic == nil {
			errs = append(errs, errors.New("ssh_dynamic settings are required"))
			break
		}

		if err := p.SSHDynamic.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("ssh_dynamic: %w", err))
		}

	case TunnelTypeKubernetesPortForward:
		if err := validatePort("local_port", p.LocalPort); err != nil {
			errs = append(errs, err)
		}

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

func (s SSHRemote) Validate() error {
	var errs []error

	if strings.TrimSpace(s.Host) == "" {
		errs = append(errs, errors.New("host is required"))
	}

	if err := validatePort("bind_port", s.BindPort); err != nil {
		errs = append(errs, err)
	}

	if strings.TrimSpace(s.TargetHost) == "" {
		errs = append(errs, errors.New("target_host is required"))
	}

	if err := validatePort("target_port", s.TargetPort); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func (s SSHDynamic) Validate() error {
	if strings.TrimSpace(s.Host) == "" {
		return errors.New("host is required")
	}

	return nil
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
