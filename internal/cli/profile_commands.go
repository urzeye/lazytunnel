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
		newProfileImportCommand(configPath),
		newProfileCloneCommand(configPath),
		newProfileEditCommand(configPath),
		newProfileRemoveCommand(configPath),
	)

	return cmd
}

func newProfileEditCommand(configPath *string) *cobra.Command {
	var (
		opts        editProfileOptions
		interactive bool
	)

	cmd := &cobra.Command{
		Use:     "edit <name>",
		Aliases: []string{"update", "set"},
		Short:   "Edit an existing tunnel profile in place",
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
			targetName := source.Name
			if cmd.Flags().Changed("name") {
				targetName = strings.TrimSpace(opts.name)
				profile.Name = targetName
			}

			if err := applyProfileEditFlags(cmd, &profile, opts); err != nil {
				return err
			}

			if interactive {
				profile, err = runInteractiveProfileEdit(cmd.InOrStdin(), cmd.OutOrStdout(), profile)
				if err != nil {
					return err
				}
				targetName = strings.TrimSpace(profile.Name)
			}

			if err := profile.Validate(); err != nil {
				return augmentProfileValidationError(sourceName, profile, err)
			}

			if targetName != sourceName {
				if _, exists := findProfile(cfg.Profiles, targetName); exists {
					return augmentProfileValidationError(sourceName, profile, fmt.Errorf("profile %q already exists", targetName))
				}
			}

			if targetName != sourceName {
				if !cfg.RemoveProfile(sourceName) {
					return fmt.Errorf("profile %q not found", sourceName)
				}
			}

			cfg.SetProfile(profile)
			if targetName != sourceName {
				cfg.RenameProfileInStacks(sourceName, targetName)
			}

			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return augmentProfileValidationError(sourceName, profile, fmt.Errorf("save config: %w", err))
			}

			if targetName != sourceName {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated profile %s (renamed from %s)\n", targetName, sourceName)
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated profile %s\n", targetName)
			return nil
		},
	}

	bindEditProfileFlags(cmd, &opts)
	cmd.Flags().BoolVar(&interactive, "interactive", false, "edit the profile through interactive prompts")

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
				assignProfileDisplayPort(&profile, localPort)
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
		newProfileAddSSHRemoteCommand(configPath),
		newProfileAddSSHDynamicCommand(configPath),
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

func newProfileAddSSHRemoteCommand(configPath *string) *cobra.Command {
	var opts addSSHRemoteOptions

	cmd := &cobra.Command{
		Use:   "ssh-remote",
		Short: "Add or update an SSH remote-forward profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile := domain.Profile{
				Name:        opts.name,
				Description: opts.description,
				Type:        domain.TunnelTypeSSHRemote,
				LocalPort:   opts.bindPort,
				Labels:      cleanList(opts.labels),
				Restart:     opts.restartPolicy(),
				SSHRemote: &domain.SSHRemote{
					Host:        opts.host,
					BindAddress: opts.bindAddress,
					BindPort:    opts.bindPort,
					TargetHost:  opts.targetHost,
					TargetPort:  opts.targetPort,
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

	bindCommonProfileFlagsWithoutLocalPort(cmd, &opts.profileOptions)
	cmd.Flags().StringVar(&opts.host, "host", "", "SSH host alias or hostname to connect to")
	cmd.Flags().StringVar(&opts.bindAddress, "bind-address", "", "remote bind address exposed by the SSH server")
	cmd.Flags().IntVar(&opts.bindPort, "bind-port", 0, "remote port to expose on the SSH server")
	cmd.Flags().StringVar(&opts.targetHost, "target-host", "", "target host reachable from the current machine")
	cmd.Flags().IntVar(&opts.targetPort, "target-port", 0, "target port reachable from the current machine")
	mustMarkRequired(cmd, "name", "host", "bind-port", "target-host", "target-port")

	return cmd
}

func newProfileAddSSHDynamicCommand(configPath *string) *cobra.Command {
	var opts addSSHDynamicOptions

	cmd := &cobra.Command{
		Use:   "ssh-dynamic",
		Short: "Add or update an SSH dynamic SOCKS profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			profile := domain.Profile{
				Name:        opts.name,
				Description: opts.description,
				Type:        domain.TunnelTypeSSHDynamic,
				LocalPort:   opts.localPort,
				Labels:      cleanList(opts.labels),
				Restart:     opts.restartPolicy(),
				SSHDynamic: &domain.SSHDynamic{
					Host:        opts.host,
					BindAddress: opts.bindAddress,
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
	cmd.Flags().StringVar(&opts.bindAddress, "bind-address", "", "local bind address exposed by the SOCKS listener")
	mustMarkRequired(cmd, "name", "host", "local-port")

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

type addSSHRemoteOptions struct {
	profileOptions
	host        string
	bindAddress string
	bindPort    int
	targetHost  string
	targetPort  int
}

type addSSHDynamicOptions struct {
	profileOptions
	host        string
	bindAddress string
}

type editProfileOptions struct {
	name           string
	description    string
	localPort      int
	labels         []string
	clearLabels    bool
	restartEnabled bool
	maxRetries     int
	initialBackoff string
	maxBackoff     string
	host           string
	remoteHost     string
	remotePort     int
	bindAddress    string
	bindPort       int
	targetHost     string
	targetPort     int
	context        string
	namespace      string
	resourceType   string
	resource       string
}

func bindCommonProfileFlags(cmd *cobra.Command, opts *profileOptions) {
	bindCommonProfileFlagsWithoutLocalPort(cmd, opts)
	cmd.Flags().IntVar(&opts.localPort, "local-port", 0, "local port to bind")
}

func bindCommonProfileFlagsWithoutLocalPort(cmd *cobra.Command, opts *profileOptions) {
	cmd.Flags().StringVar(&opts.name, "name", "", "profile name")
	cmd.Flags().StringVar(&opts.description, "description", "", "optional profile description")
	cmd.Flags().StringSliceVar(&opts.labels, "label", nil, "labels to attach to the profile")
	cmd.Flags().BoolVar(&opts.overwrite, "overwrite", false, "overwrite an existing profile with the same name")
	cmd.Flags().BoolVar(&opts.restartEnabled, "restart", true, "restart the profile when the process exits unexpectedly")
	cmd.Flags().IntVar(&opts.maxRetries, "max-retries", 0, "maximum restart attempts; 0 means unlimited while restart is enabled")
	cmd.Flags().StringVar(&opts.initialBackoff, "initial-backoff", "2s", "initial restart backoff duration")
	cmd.Flags().StringVar(&opts.maxBackoff, "max-backoff", "30s", "maximum restart backoff duration")
}

func bindEditProfileFlags(cmd *cobra.Command, opts *editProfileOptions) {
	cmd.Flags().StringVar(&opts.name, "name", "", "rename the profile")
	cmd.Flags().StringVar(&opts.description, "description", "", "update the profile description")
	cmd.Flags().IntVar(&opts.localPort, "local-port", 0, "update the local port")
	cmd.Flags().StringSliceVar(&opts.labels, "label", nil, "replace labels on the profile")
	cmd.Flags().BoolVar(&opts.clearLabels, "clear-labels", false, "remove all labels before applying label overrides")
	cmd.Flags().BoolVar(&opts.restartEnabled, "restart", false, "update whether the profile should restart automatically")
	cmd.Flags().IntVar(&opts.maxRetries, "max-retries", 0, "update the maximum restart attempts; 0 means unlimited")
	cmd.Flags().StringVar(&opts.initialBackoff, "initial-backoff", "", "update the initial restart backoff duration")
	cmd.Flags().StringVar(&opts.maxBackoff, "max-backoff", "", "update the maximum restart backoff duration")
	cmd.Flags().StringVar(&opts.host, "host", "", "update the SSH host alias or hostname")
	cmd.Flags().StringVar(&opts.remoteHost, "remote-host", "", "update the SSH remote host")
	cmd.Flags().IntVar(&opts.remotePort, "remote-port", 0, "update the SSH or Kubernetes remote port")
	cmd.Flags().StringVar(&opts.bindAddress, "bind-address", "", "update the SSH remote bind address")
	cmd.Flags().IntVar(&opts.bindPort, "bind-port", 0, "update the SSH remote bind port")
	cmd.Flags().StringVar(&opts.targetHost, "target-host", "", "update the SSH remote target host")
	cmd.Flags().IntVar(&opts.targetPort, "target-port", 0, "update the SSH remote target port")
	cmd.Flags().StringVar(&opts.context, "context", "", "update the Kubernetes context name")
	cmd.Flags().StringVar(&opts.namespace, "namespace", "", "update the Kubernetes namespace")
	cmd.Flags().StringVar(&opts.resourceType, "resource-type", "", "update the Kubernetes resource type")
	cmd.Flags().StringVar(&opts.resource, "resource", "", "update the Kubernetes resource name")
}

func applyProfileEditFlags(cmd *cobra.Command, profile *domain.Profile, opts editProfileOptions) error {
	if cmd.Flags().Changed("description") {
		profile.Description = opts.description
	}
	if cmd.Flags().Changed("local-port") {
		assignProfileDisplayPort(profile, opts.localPort)
	}
	if opts.clearLabels {
		profile.Labels = nil
	}
	if cmd.Flags().Changed("label") {
		profile.Labels = cleanList(opts.labels)
	}
	if cmd.Flags().Changed("restart") {
		profile.Restart.Enabled = opts.restartEnabled
	}
	if cmd.Flags().Changed("max-retries") {
		profile.Restart.MaxRetries = opts.maxRetries
	}
	if cmd.Flags().Changed("initial-backoff") {
		profile.Restart.InitialBackoff = opts.initialBackoff
	}
	if cmd.Flags().Changed("max-backoff") {
		profile.Restart.MaxBackoff = opts.maxBackoff
	}

	switch profile.Type {
	case domain.TunnelTypeSSHLocal:
		if cmd.Flags().Changed("context") || cmd.Flags().Changed("namespace") || cmd.Flags().Changed("resource-type") || cmd.Flags().Changed("resource") {
			return fmt.Errorf("kubernetes-specific flags cannot be used when editing SSH profile %q", profile.Name)
		}
		if cmd.Flags().Changed("bind-address") || cmd.Flags().Changed("bind-port") || cmd.Flags().Changed("target-host") || cmd.Flags().Changed("target-port") {
			return fmt.Errorf("ssh-remote-specific flags cannot be used when editing SSH local profile %q", profile.Name)
		}

		if profile.SSH == nil {
			profile.SSH = &domain.SSHLocal{}
		}
		if cmd.Flags().Changed("host") {
			profile.SSH.Host = opts.host
		}
		if cmd.Flags().Changed("remote-host") {
			profile.SSH.RemoteHost = opts.remoteHost
		}
		if cmd.Flags().Changed("remote-port") {
			profile.SSH.RemotePort = opts.remotePort
		}

	case domain.TunnelTypeSSHRemote:
		if cmd.Flags().Changed("context") || cmd.Flags().Changed("namespace") || cmd.Flags().Changed("resource-type") || cmd.Flags().Changed("resource") {
			return fmt.Errorf("kubernetes-specific flags cannot be used when editing SSH remote profile %q", profile.Name)
		}
		if cmd.Flags().Changed("remote-host") || cmd.Flags().Changed("remote-port") {
			return fmt.Errorf("ssh-local-specific flags cannot be used when editing SSH remote profile %q", profile.Name)
		}
		if cmd.Flags().Changed("local-port") {
			return fmt.Errorf("use --bind-port when editing SSH remote profile %q", profile.Name)
		}

		if profile.SSHRemote == nil {
			profile.SSHRemote = &domain.SSHRemote{}
		}
		if cmd.Flags().Changed("host") {
			profile.SSHRemote.Host = opts.host
		}
		if cmd.Flags().Changed("bind-address") {
			profile.SSHRemote.BindAddress = opts.bindAddress
		}
		if cmd.Flags().Changed("bind-port") {
			profile.SSHRemote.BindPort = opts.bindPort
			profile.LocalPort = opts.bindPort
		}
		if cmd.Flags().Changed("target-host") {
			profile.SSHRemote.TargetHost = opts.targetHost
		}
		if cmd.Flags().Changed("target-port") {
			profile.SSHRemote.TargetPort = opts.targetPort
		}

	case domain.TunnelTypeSSHDynamic:
		if cmd.Flags().Changed("context") || cmd.Flags().Changed("namespace") || cmd.Flags().Changed("resource-type") || cmd.Flags().Changed("resource") {
			return fmt.Errorf("kubernetes-specific flags cannot be used when editing SSH dynamic profile %q", profile.Name)
		}
		if cmd.Flags().Changed("remote-host") || cmd.Flags().Changed("remote-port") {
			return fmt.Errorf("ssh-local-specific flags cannot be used when editing SSH dynamic profile %q", profile.Name)
		}
		if cmd.Flags().Changed("target-host") || cmd.Flags().Changed("target-port") {
			return fmt.Errorf("ssh-remote-specific flags cannot be used when editing SSH dynamic profile %q", profile.Name)
		}
		if cmd.Flags().Changed("bind-port") {
			return fmt.Errorf("use --local-port when editing SSH dynamic profile %q", profile.Name)
		}

		if profile.SSHDynamic == nil {
			profile.SSHDynamic = &domain.SSHDynamic{}
		}
		if cmd.Flags().Changed("host") {
			profile.SSHDynamic.Host = opts.host
		}
		if cmd.Flags().Changed("bind-address") {
			profile.SSHDynamic.BindAddress = opts.bindAddress
		}

	case domain.TunnelTypeKubernetesPortForward:
		if cmd.Flags().Changed("host") || cmd.Flags().Changed("remote-host") || cmd.Flags().Changed("bind-address") || cmd.Flags().Changed("bind-port") || cmd.Flags().Changed("target-host") || cmd.Flags().Changed("target-port") {
			return fmt.Errorf("ssh-specific flags cannot be used when editing Kubernetes profile %q", profile.Name)
		}

		if profile.Kubernetes == nil {
			profile.Kubernetes = &domain.Kubernetes{}
		}
		if cmd.Flags().Changed("context") {
			profile.Kubernetes.Context = opts.context
		}
		if cmd.Flags().Changed("namespace") {
			profile.Kubernetes.Namespace = opts.namespace
		}
		if cmd.Flags().Changed("resource-type") {
			profile.Kubernetes.ResourceType = opts.resourceType
		}
		if cmd.Flags().Changed("resource") {
			profile.Kubernetes.Resource = opts.resource
		}
		if cmd.Flags().Changed("remote-port") {
			profile.Kubernetes.RemotePort = opts.remotePort
		}
	}

	return nil
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
	if profile.SSHRemote != nil {
		sshRemoteCopy := *profile.SSHRemote
		cloned.SSHRemote = &sshRemoteCopy
	}
	if profile.SSHDynamic != nil {
		sshDynamicCopy := *profile.SSHDynamic
		cloned.SSHDynamic = &sshDynamicCopy
	}
	if profile.Kubernetes != nil {
		kubernetesCopy := *profile.Kubernetes
		cloned.Kubernetes = &kubernetesCopy
	}

	return cloned
}

func assignProfileDisplayPort(profile *domain.Profile, port int) {
	profile.LocalPort = port
	if profile.Type == domain.TunnelTypeSSHRemote && profile.SSHRemote != nil {
		profile.SSHRemote.BindPort = port
	}
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
