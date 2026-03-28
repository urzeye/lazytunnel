package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/urzeye/lazytunnel/internal/domain"
	"github.com/urzeye/lazytunnel/internal/storage"
)

func newProfileCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage tunnel profiles",
	}

	cmd.AddCommand(
		newProfileListCommand(configPath),
		newProfileAddCommand(configPath),
		newProfileCloneCommand(configPath),
		newProfileRemoveCommand(configPath),
	)

	return cmd
}

func newProfileCloneCommand(configPath *string) *cobra.Command {
	var (
		name        string
		description string
		localPort   int
		labels      []string
		clearLabels bool
		overwrite   bool
	)

	cmd := &cobra.Command{
		Use:     "clone <source>",
		Aliases: []string{"copy", "cp", "duplicate"},
		Short:   "Clone an existing tunnel profile into a new one",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceName := strings.TrimSpace(args[0])

			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			source, exists := findProfile(cfg.Profiles, sourceName)
			if !exists {
				return fmt.Errorf("profile %q not found", sourceName)
			}

			profile := cloneProfile(source)
			profile.Name = name

			if cmd.Flags().Changed("description") {
				profile.Description = description
			}
			if cmd.Flags().Changed("local-port") {
				profile.LocalPort = localPort
			}
			if clearLabels {
				profile.Labels = nil
			}
			if cmd.Flags().Changed("label") {
				profile.Labels = cleanList(labels)
			}

			created, err := saveProfileConfig(*configPath, cfg, profile, overwrite)
			if err != nil {
				return err
			}

			action := "updated"
			if created {
				action = "cloned"
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s profile %s from %s\n", action, profile.Name, sourceName)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new profile name")
	cmd.Flags().StringVar(&description, "description", "", "override the description on the cloned profile")
	cmd.Flags().IntVar(&localPort, "local-port", 0, "override the local port on the cloned profile")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "replace labels on the cloned profile")
	cmd.Flags().BoolVar(&clearLabels, "clear-labels", false, "remove all labels from the cloned profile before applying overrides")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite an existing profile with the same target name")
	mustMarkRequired(cmd, "name")

	return cmd
}

func newProfileAddCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new tunnel profile",
	}

	cmd.AddCommand(
		newProfileAddSSHLocalCommand(configPath),
		newProfileAddKubernetesCommand(configPath),
	)

	return cmd
}

func newProfileRemoveCommand(configPath *string) *cobra.Command {
	var removeFromStacks bool

	cmd := &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "delete", "del"},
		Short:   "Remove a configured profile",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])

			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if !cfg.RemoveProfile(name) {
				return fmt.Errorf("profile %q not found", name)
			}

			referencing := cfg.StacksReferencingProfile(name)
			if len(referencing) > 0 {
				if !removeFromStacks {
					return fmt.Errorf(
						"profile %q is still referenced by stacks: %s; rerun with --remove-from-stacks to prune those references",
						name,
						strings.Join(referencing, ", "),
					)
				}

				updatedStacks, removedStacks := cfg.RemoveProfileFromStacks(name)
				if err := storage.SaveConfig(*configPath, cfg); err != nil {
					return fmt.Errorf("save config: %w", err)
				}

				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"removed profile %s and pruned %d stack references (%d empty stacks removed)\n",
					name,
					updatedStacks,
					removedStacks,
				)
				return nil
			}

			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed profile %s\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&removeFromStacks, "remove-from-stacks", false, "remove the profile from any stacks that still reference it")

	return cmd
}

func newProfileListCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if len(cfg.Profiles) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no profiles configured")
				return nil
			}

			for _, profile := range cfg.Profiles {
				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%s\t:%d\n",
					profile.Name,
					profile.Type,
					profile.LocalPort,
				)
			}
			return nil
		},
	}
}

func newProfileAddSSHLocalCommand(configPath *string) *cobra.Command {
	var opts addSSHLocalOptions

	cmd := &cobra.Command{
		Use:   "ssh-local",
		Short: "Add or update an SSH local-forward profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile := domain.Profile{
				Name:        opts.name,
				Description: opts.description,
				Type:        domain.TunnelTypeSSHLocal,
				LocalPort:   opts.localPort,
				Labels:      cleanList(opts.labels),
				Restart:     opts.restartPolicy(),
				SSH: &domain.SSHLocal{
					Host:       opts.host,
					RemoteHost: opts.remoteHost,
					RemotePort: opts.remotePort,
				},
			}

			created, err := saveProfile(*configPath, profile, opts.overwrite)
			if err != nil {
				return err
			}

			action := "updated"
			if created {
				action = "added"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s profile %s\n", action, profile.Name)
			return nil
		},
	}

	bindCommonProfileFlags(cmd, &opts.profileOptions)
	cmd.Flags().StringVar(&opts.host, "host", "", "SSH host alias or hostname to connect to")
	cmd.Flags().StringVar(&opts.remoteHost, "remote-host", "", "remote host to forward traffic to")
	cmd.Flags().IntVar(&opts.remotePort, "remote-port", 0, "remote port to forward traffic to")
	mustMarkRequired(cmd, "name", "host", "remote-host", "remote-port", "local-port")

	return cmd
}

func newProfileAddKubernetesCommand(configPath *string) *cobra.Command {
	var opts addKubernetesOptions

	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: "Add or update a Kubernetes port-forward profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile := domain.Profile{
				Name:        opts.name,
				Description: opts.description,
				Type:        domain.TunnelTypeKubernetesPortForward,
				LocalPort:   opts.localPort,
				Labels:      cleanList(opts.labels),
				Restart:     opts.restartPolicy(),
				Kubernetes: &domain.Kubernetes{
					Context:      opts.context,
					Namespace:    opts.namespace,
					ResourceType: opts.resourceType,
					Resource:     opts.resource,
					RemotePort:   opts.remotePort,
				},
			}

			created, err := saveProfile(*configPath, profile, opts.overwrite)
			if err != nil {
				return err
			}

			action := "updated"
			if created {
				action = "added"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s profile %s\n", action, profile.Name)
			return nil
		},
	}

	bindCommonProfileFlags(cmd, &opts.profileOptions)
	cmd.Flags().StringVar(&opts.context, "context", "", "Kubernetes context name")
	cmd.Flags().StringVar(&opts.namespace, "namespace", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&opts.resourceType, "resource-type", "", "resource type: pod, service, or deployment")
	cmd.Flags().StringVar(&opts.resource, "resource", "", "resource name to port-forward")
	cmd.Flags().IntVar(&opts.remotePort, "remote-port", 0, "remote port exposed by the Kubernetes resource")
	mustMarkRequired(cmd, "name", "resource-type", "resource", "remote-port", "local-port")

	return cmd
}

type profileOptions struct {
	name           string
	description    string
	localPort      int
	labels         []string
	overwrite      bool
	restartEnabled bool
	maxRetries     int
	initialBackoff string
	maxBackoff     string
}

func (o profileOptions) restartPolicy() domain.RestartPolicy {
	return domain.RestartPolicy{
		Enabled:        o.restartEnabled,
		MaxRetries:     o.maxRetries,
		InitialBackoff: o.initialBackoff,
		MaxBackoff:     o.maxBackoff,
	}
}

type addSSHLocalOptions struct {
	profileOptions
	host       string
	remoteHost string
	remotePort int
}

type addKubernetesOptions struct {
	profileOptions
	context      string
	namespace    string
	resourceType string
	resource     string
	remotePort   int
}

func bindCommonProfileFlags(cmd *cobra.Command, opts *profileOptions) {
	cmd.Flags().StringVar(&opts.name, "name", "", "profile name")
	cmd.Flags().StringVar(&opts.description, "description", "", "optional profile description")
	cmd.Flags().IntVar(&opts.localPort, "local-port", 0, "local port to bind")
	cmd.Flags().StringSliceVar(&opts.labels, "label", nil, "labels to attach to the profile")
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", false, "overwrite an existing profile with the same name")
	cmd.Flags().BoolVar(&opts.restartEnabled, "restart", true, "restart the profile when the process exits unexpectedly")
	cmd.Flags().IntVar(&opts.maxRetries, "max-retries", 0, "maximum restart attempts; 0 means unlimited while restart is enabled")
	cmd.Flags().StringVar(&opts.initialBackoff, "initial-backoff", "2s", "initial restart backoff duration")
	cmd.Flags().StringVar(&opts.maxBackoff, "max-backoff", "30s", "maximum restart backoff duration")
}

func saveProfile(configPath string, profile domain.Profile, overwrite bool) (bool, error) {
	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}

	return saveProfileConfig(configPath, cfg, profile, overwrite)
}

func saveProfileConfig(configPath string, cfg domain.Config, profile domain.Profile, overwrite bool) (bool, error) {
	exists := false
	for _, existing := range cfg.Profiles {
		if existing.Name == profile.Name {
			exists = true
			break
		}
	}

	if exists && !overwrite {
		return false, fmt.Errorf("profile %q already exists; use --overwrite to update it", profile.Name)
	}

	created := cfg.SetProfile(profile)
	if err := storage.SaveConfig(configPath, cfg); err != nil {
		return false, fmt.Errorf("save config: %w", err)
	}

	return created, nil
}

func findProfile(profiles []domain.Profile, name string) (domain.Profile, bool) {
	for _, profile := range profiles {
		if profile.Name == name {
			return profile, true
		}
	}

	return domain.Profile{}, false
}

func cloneProfile(profile domain.Profile) domain.Profile {
	cloned := profile
	cloned.Labels = append([]string(nil), profile.Labels...)
	if profile.SSH != nil {
		sshCopy := *profile.SSH
		cloned.SSH = &sshCopy
	}
	if profile.Kubernetes != nil {
		kubernetesCopy := *profile.Kubernetes
		cloned.Kubernetes = &kubernetesCopy
	}

	return cloned
}

func mustMarkRequired(cmd *cobra.Command, names ...string) {
	for _, name := range names {
		if err := cmd.MarkFlagRequired(name); err != nil {
			panic(err)
		}
	}
}

func cleanList(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		cleaned = append(cleaned, value)
	}
	return cleaned
}
