package profileimport

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/urzeye/lazytunnel/internal/domain"
)

type Result struct {
	SourcePath   string
	Created      int
	Updated      int
	Skipped      int
	ProfileNames []string
}

type sshConfigEntry struct {
	Alias    string
	HostName string
	User     string
	Port     int
	Source   string
}

type kubeconfigFile struct {
	CurrentContext string              `yaml:"current-context"`
	Contexts       []kubeconfigContext `yaml:"contexts"`
}

type kubeconfigContext struct {
	Name    string                `yaml:"name"`
	Context kubeconfigContextSpec `yaml:"context"`
}

type kubeconfigContextSpec struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace"`
}

func DefaultSSHConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/config"
	}

	return filepath.Join(homeDir, ".ssh", "config")
}

func DefaultKubeconfigPath() string {
	kubeconfig := strings.TrimSpace(os.Getenv("KUBECONFIG"))
	if kubeconfig != "" {
		parts := filepath.SplitList(kubeconfig)
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				continue
			}
			return part
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.kube/config"
	}

	return filepath.Join(homeDir, ".kube", "config")
}

func ImportSSHConfig(cfg domain.Config, path string, overwrite bool) (domain.Config, Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultSSHConfigPath()
	}

	entries, resolvedPath, err := loadSSHConfigEntries(path)
	if err != nil {
		return cfg, Result{}, err
	}
	if len(entries) == 0 {
		return cfg, Result{}, fmt.Errorf("no concrete SSH hosts found in %q", resolvedPath)
	}

	updated := cloneConfig(cfg)
	usedPorts := usedLocalPorts(updated)
	result := Result{
		SourcePath:   resolvedPath,
		ProfileNames: make([]string, 0, len(entries)),
	}

	for _, entry := range entries {
		if profileExists(updated.Profiles, entry.Alias) && !overwrite {
			result.Skipped++
			continue
		}

		profile := importedSSHProfile(entry, usedPorts)
		if updated.SetProfile(profile) {
			result.Created++
		} else {
			result.Updated++
		}
		result.ProfileNames = append(result.ProfileNames, profile.Name)
	}

	return updated, result, nil
}

func ImportKubeContexts(cfg domain.Config, path string, overwrite bool) (domain.Config, Result, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultKubeconfigPath()
	}

	kubeconfig, resolvedPath, err := loadKubeconfig(path)
	if err != nil {
		return cfg, Result{}, err
	}
	if len(kubeconfig.Contexts) == 0 {
		return cfg, Result{}, fmt.Errorf("no Kubernetes contexts found in %q", resolvedPath)
	}

	updated := cloneConfig(cfg)
	usedPorts := usedLocalPorts(updated)
	result := Result{
		SourcePath:   resolvedPath,
		ProfileNames: make([]string, 0, len(kubeconfig.Contexts)),
	}

	for _, context := range kubeconfig.Contexts {
		if profileExists(updated.Profiles, context.Name) && !overwrite {
			result.Skipped++
			continue
		}

		profile := importedKubernetesProfile(resolvedPath, kubeconfig.CurrentContext, context, usedPorts)
		if updated.SetProfile(profile) {
			result.Created++
		} else {
			result.Updated++
		}
		result.ProfileNames = append(result.ProfileNames, profile.Name)
	}

	return updated, result, nil
}

func loadSSHConfigEntries(path string) ([]sshConfigEntry, string, error) {
	entriesByAlias := make(map[string]sshConfigEntry)
	order := make([]string, 0)
	visited := make(map[string]struct{})

	resolvedPath, err := expandUserPath(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve SSH config path %q: %w", path, err)
	}

	if err := collectSSHConfigEntries(resolvedPath, visited, entriesByAlias, &order); err != nil {
		return nil, "", err
	}

	entries := make([]sshConfigEntry, 0, len(order))
	for _, alias := range order {
		entries = append(entries, entriesByAlias[alias])
	}

	return entries, resolvedPath, nil
}

func loadKubeconfig(path string) (kubeconfigFile, string, error) {
	resolvedPath, err := expandUserPath(path)
	if err != nil {
		return kubeconfigFile{}, "", fmt.Errorf("resolve kubeconfig path %q: %w", path, err)
	}

	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return kubeconfigFile{}, "", fmt.Errorf("read kubeconfig %q: %w", resolvedPath, err)
	}

	var kubeconfig kubeconfigFile
	if err := yaml.Unmarshal(content, &kubeconfig); err != nil {
		return kubeconfigFile{}, "", fmt.Errorf("decode kubeconfig %q: %w", resolvedPath, err)
	}

	return kubeconfig, resolvedPath, nil
}

func collectSSHConfigEntries(path string, visited map[string]struct{}, entriesByAlias map[string]sshConfigEntry, order *[]string) error {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve SSH config path %q: %w", path, err)
	}
	if _, exists := visited[absolutePath]; exists {
		return nil
	}
	visited[absolutePath] = struct{}{}

	file, err := os.Open(absolutePath)
	if err != nil {
		return fmt.Errorf("open SSH config %q: %w", absolutePath, err)
	}
	defer file.Close()

	type sshBlock struct {
		Aliases  []string
		HostName string
		User     string
		Port     int
	}

	var current *sshBlock
	flushCurrent := func() {
		if current == nil {
			return
		}

		for _, alias := range current.Aliases {
			if _, exists := entriesByAlias[alias]; exists {
				continue
			}
			entriesByAlias[alias] = sshConfigEntry{
				Alias:    alias,
				HostName: current.HostName,
				User:     current.User,
				Port:     current.Port,
				Source:   absolutePath,
			}
			*order = append(*order, alias)
		}

		current = nil
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(stripSSHConfigComment(scanner.Text()))
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		key := strings.ToLower(fields[0])
		values := fields[1:]

		switch key {
		case "include":
			for _, pattern := range values {
				if err := importSSHIncludePattern(filepath.Dir(absolutePath), pattern, visited, entriesByAlias, order); err != nil {
					return err
				}
			}
		case "host":
			flushCurrent()
			aliases := concreteSSHHostAliases(values)
			if len(aliases) == 0 {
				continue
			}
			current = &sshBlock{Aliases: aliases}
		case "match":
			flushCurrent()
		case "hostname":
			if current != nil && current.HostName == "" && len(values) > 0 {
				current.HostName = values[0]
			}
		case "user":
			if current != nil && current.User == "" && len(values) > 0 {
				current.User = values[0]
			}
		case "port":
			if current != nil && current.Port == 0 && len(values) > 0 {
				if port, err := strconv.Atoi(values[0]); err == nil && port > 0 {
					current.Port = port
				}
			}
		}
	}

	flushCurrent()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read SSH config %q: %w", absolutePath, err)
	}

	return nil
}

func importSSHIncludePattern(baseDir, pattern string, visited map[string]struct{}, entriesByAlias map[string]sshConfigEntry, order *[]string) error {
	resolvedPattern, err := expandUserPath(pattern)
	if err != nil {
		return fmt.Errorf("resolve SSH include %q: %w", pattern, err)
	}
	if !filepath.IsAbs(resolvedPattern) {
		resolvedPattern = filepath.Join(baseDir, resolvedPattern)
	}

	matches, err := filepath.Glob(resolvedPattern)
	if err != nil {
		return fmt.Errorf("glob SSH include %q: %w", pattern, err)
	}
	slices.Sort(matches)

	for _, match := range matches {
		if err := collectSSHConfigEntries(match, visited, entriesByAlias, order); err != nil {
			return err
		}
	}

	return nil
}

func stripSSHConfigComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func concreteSSHHostAliases(values []string) []string {
	aliases := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.HasPrefix(value, "!") {
			continue
		}
		if strings.ContainsAny(value, "*?") {
			continue
		}
		aliases = append(aliases, value)
	}
	return aliases
}

func importedSSHProfile(entry sshConfigEntry, usedPorts map[int]struct{}) domain.Profile {
	localPort := nextAvailableImportedPort(usedPorts, 15432)

	details := []string{
		fmt.Sprintf("Imported from %s.", entry.Source),
	}
	if entry.HostName != "" {
		details = append(details, fmt.Sprintf("HostName %s.", entry.HostName))
	}
	if entry.User != "" {
		details = append(details, fmt.Sprintf("User %s.", entry.User))
	}
	if entry.Port > 0 {
		details = append(details, fmt.Sprintf("SSH port %d.", entry.Port))
	}
	details = append(details, "Update the forward target before using this draft.")

	return domain.Profile{
		Name:        entry.Alias,
		Description: strings.Join(details, " "),
		Type:        domain.TunnelTypeSSHLocal,
		LocalPort:   localPort,
		Labels:      []string{"draft", "imported", "ssh-config"},
		Restart: domain.RestartPolicy{
			Enabled:        true,
			MaxRetries:     0,
			InitialBackoff: "2s",
			MaxBackoff:     "30s",
		},
		SSH: &domain.SSHLocal{
			Host:       entry.Alias,
			RemoteHost: "127.0.0.1",
			RemotePort: 80,
		},
	}
}

func importedKubernetesProfile(sourcePath, currentContext string, context kubeconfigContext, usedPorts map[int]struct{}) domain.Profile {
	localPort := nextAvailableImportedPort(usedPorts, 18080)
	labels := []string{"draft", "imported", "kube-context"}
	if context.Name == currentContext && currentContext != "" {
		labels = append(labels, "current-context")
	}

	details := []string{
		fmt.Sprintf("Imported from %s.", sourcePath),
		fmt.Sprintf("Kubernetes context %s.", context.Name),
	}
	if context.Context.Cluster != "" {
		details = append(details, fmt.Sprintf("Cluster %s.", context.Context.Cluster))
	}
	if context.Context.User != "" {
		details = append(details, fmt.Sprintf("User %s.", context.Context.User))
	}
	if context.Context.Namespace != "" {
		details = append(details, fmt.Sprintf("Namespace %s.", context.Context.Namespace))
	}
	details = append(details, "Update the resource target before using this draft.")

	return domain.Profile{
		Name:        context.Name,
		Description: strings.Join(details, " "),
		Type:        domain.TunnelTypeKubernetesPortForward,
		LocalPort:   localPort,
		Labels:      labels,
		Restart: domain.RestartPolicy{
			Enabled:        true,
			MaxRetries:     0,
			InitialBackoff: "2s",
			MaxBackoff:     "30s",
		},
		Kubernetes: &domain.Kubernetes{
			Context:      context.Name,
			Namespace:    context.Context.Namespace,
			ResourceType: "service",
			Resource:     "change-me",
			RemotePort:   80,
		},
	}
}

func usedLocalPorts(cfg domain.Config) map[int]struct{} {
	used := make(map[int]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		used[profile.LocalPort] = struct{}{}
	}
	return used
}

func nextAvailableImportedPort(used map[int]struct{}, base int) int {
	port := base
	for {
		if _, exists := used[port]; !exists {
			used[port] = struct{}{}
			return port
		}
		port++
	}
}

func profileExists(profiles []domain.Profile, name string) bool {
	for _, profile := range profiles {
		if profile.Name == name {
			return true
		}
	}
	return false
}

func cloneConfig(cfg domain.Config) domain.Config {
	cloned := cfg
	cloned.Profiles = append([]domain.Profile(nil), cfg.Profiles...)
	cloned.Stacks = append([]domain.Stack(nil), cfg.Stacks...)
	return cloned
}

func expandUserPath(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return homeDir, nil
	}

	return filepath.Join(homeDir, strings.TrimPrefix(path, "~/")), nil
}
