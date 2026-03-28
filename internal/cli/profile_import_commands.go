package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/urzeye/lazytunnel/internal/domain"
	"github.com/urzeye/lazytunnel/internal/storage"
)

type sshConfigImportEntry struct {
	Alias    string
	HostName string
	User     string
	Port     int
	Source   string
}

func newProfileImportCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import draft profiles from existing SSH or Kubernetes config",
	}

	cmd.AddCommand(
		newProfileImportSSHConfigCommand(configPath),
	)

	return cmd
}

func newProfileImportSSHConfigCommand(configPath *string) *cobra.Command {
	var (
		path      string
		overwrite bool
	)

	cmd := &cobra.Command{
		Use:   "ssh-config",
		Short: "Import SSH host aliases from ~/.ssh/config as draft profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(path) == "" {
				path = defaultSSHConfigPath()
			}

			entries, err := loadSSHConfigImportEntries(path)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				return fmt.Errorf("no concrete SSH hosts found in %q", path)
			}

			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			usedPorts := usedLocalPorts(cfg)
			created := 0
			updated := 0
			skipped := 0
			importedNames := make([]string, 0, len(entries))

			for _, entry := range entries {
				if _, exists := findProfile(cfg.Profiles, entry.Alias); exists && !overwrite {
					skipped++
					continue
				}

				profile := importedSSHProfile(entry, usedPorts)
				if cfg.SetProfile(profile) {
					created++
				} else {
					updated++
				}
				importedNames = append(importedNames, profile.Name)
			}

			if created == 0 && updated == 0 {
				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"imported SSH config from %s: 0 created, 0 updated, %d skipped\n",
					path,
					skipped,
				)
				return nil
			}

			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(),
				"imported SSH config from %s: %d created, %d updated, %d skipped\n",
				path,
				created,
				updated,
				skipped,
			)
			if len(importedNames) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "profiles: %s\n", strings.Join(importedNames, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "path to the SSH config file to import")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite existing profiles when imported names collide")

	return cmd
}

func defaultSSHConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "~/.ssh/config"
	}

	return filepath.Join(homeDir, ".ssh", "config")
}

func loadSSHConfigImportEntries(path string) ([]sshConfigImportEntry, error) {
	entriesByAlias := make(map[string]sshConfigImportEntry)
	order := make([]string, 0)
	visited := make(map[string]struct{})

	resolvedPath, err := expandUserPath(path)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH config path %q: %w", path, err)
	}

	if err := collectSSHConfigImportEntries(resolvedPath, visited, entriesByAlias, &order); err != nil {
		return nil, err
	}

	entries := make([]sshConfigImportEntry, 0, len(order))
	for _, alias := range order {
		entries = append(entries, entriesByAlias[alias])
	}

	return entries, nil
}

func collectSSHConfigImportEntries(path string, visited map[string]struct{}, entriesByAlias map[string]sshConfigImportEntry, order *[]string) error {
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
			entriesByAlias[alias] = sshConfigImportEntry{
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

func importSSHIncludePattern(baseDir, pattern string, visited map[string]struct{}, entriesByAlias map[string]sshConfigImportEntry, order *[]string) error {
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
		if err := collectSSHConfigImportEntries(match, visited, entriesByAlias, order); err != nil {
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

func importedSSHProfile(entry sshConfigImportEntry, usedPorts map[int]struct{}) domain.Profile {
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
