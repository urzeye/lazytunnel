package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/urzeye/lazytunnel/internal/domain"
	"github.com/urzeye/lazytunnel/internal/storage"
)

func newStackCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Manage tunnel stacks",
	}

	cmd.AddCommand(
		newStackListCommand(configPath),
		newStackAddCommand(configPath),
		newStackCloneCommand(configPath),
		newStackEditCommand(configPath),
		newStackRemoveCommand(configPath),
	)

	return cmd
}

func newStackEditCommand(configPath *string) *cobra.Command {
	var (
		name        string
		description string
		labels      []string
		profiles    []string
		clearLabels bool
	)

	cmd := &cobra.Command{
		Use:     "edit <name>",
		Aliases: []string{"update", "set"},
		Short:   "Edit an existing stack in place",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceName := strings.TrimSpace(args[0])

			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			source, exists := findStack(cfg.Stacks, sourceName)
			if !exists {
				return fmt.Errorf("stack %q not found", sourceName)
			}

			stack := cloneStack(source)
			targetName := source.Name
			if cmd.Flags().Changed("name") {
				targetName = strings.TrimSpace(name)
				stack.Name = targetName
			}
			if cmd.Flags().Changed("description") {
				stack.Description = description
			}
			if clearLabels {
				stack.Labels = nil
			}
			if cmd.Flags().Changed("label") {
				stack.Labels = cleanList(labels)
			}
			if cmd.Flags().Changed("profile") {
				stack.Profiles = cleanList(profiles)
			}

			if targetName != sourceName {
				if _, exists := findStack(cfg.Stacks, targetName); exists {
					return fmt.Errorf("stack %q already exists", targetName)
				}
				if !cfg.RemoveStack(sourceName) {
					return fmt.Errorf("stack %q not found", sourceName)
				}
			}

			cfg.SetStack(stack)
			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			if targetName != sourceName {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated stack %s (renamed from %s)\n", targetName, sourceName)
				return nil
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "updated stack %s\n", targetName)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "rename the stack")
	cmd.Flags().StringVar(&description, "description", "", "update the stack description")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "replace labels on the stack")
	cmd.Flags().StringSliceVar(&profiles, "profile", nil, "replace the member profile list on the stack")
	cmd.Flags().BoolVar(&clearLabels, "clear-labels", false, "remove all labels before applying label overrides")

	return cmd
}

func newStackCloneCommand(configPath *string) *cobra.Command {
	var (
		name        string
		description string
		labels      []string
		profiles    []string
		clearLabels bool
		overwrite   bool
	)

	cmd := &cobra.Command{
		Use:     "clone <source>",
		Aliases: []string{"copy", "cp", "duplicate"},
		Short:   "Clone an existing stack into a new one",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceName := strings.TrimSpace(args[0])

			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			source, exists := findStack(cfg.Stacks, sourceName)
			if !exists {
				return fmt.Errorf("stack %q not found", sourceName)
			}

			stack := cloneStack(source)
			stack.Name = name

			if cmd.Flags().Changed("description") {
				stack.Description = description
			}
			if clearLabels {
				stack.Labels = nil
			}
			if cmd.Flags().Changed("label") {
				stack.Labels = cleanList(labels)
			}
			if cmd.Flags().Changed("profile") {
				stack.Profiles = cleanList(profiles)
			}

			created, err := saveStackConfig(*configPath, cfg, stack, overwrite)
			if err != nil {
				return err
			}

			action := "updated"
			if created {
				action = "cloned"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s stack %s from %s\n", action, stack.Name, sourceName)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "new stack name")
	cmd.Flags().StringVar(&description, "description", "", "override the description on the cloned stack")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "replace labels on the cloned stack")
	cmd.Flags().StringSliceVar(&profiles, "profile", nil, "replace the member profile list on the cloned stack")
	cmd.Flags().BoolVar(&clearLabels, "clear-labels", false, "remove all labels from the cloned stack before applying overrides")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite an existing stack with the same target name")
	mustMarkRequired(cmd, "name")

	return cmd
}

func newStackRemoveCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "delete", "del"},
		Short:   "Remove a configured stack",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if !cfg.RemoveStack(name) {
				return fmt.Errorf("stack %q not found", name)
			}

			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed stack %s\n", name)
			return nil
		},
	}
}

func newStackListCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured stacks",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if len(cfg.Stacks) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no stacks configured")
				return nil
			}

			for _, stack := range cfg.Stacks {
				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s\t%d members\n",
					stack.Name,
					len(stack.Profiles),
				)
			}
			return nil
		},
	}
}

func newStackAddCommand(configPath *string) *cobra.Command {
	var (
		name        string
		description string
		labels      []string
		profiles    []string
		overwrite   bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add or update a stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			stack := domain.Stack{
				Name:        name,
				Description: description,
				Labels:      cleanList(labels),
				Profiles:    cleanList(profiles),
			}

			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			created, err := saveStackConfig(*configPath, cfg, stack, overwrite)
			if err != nil {
				return err
			}

			action := "updated"
			if created {
				action = "added"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s stack %s\n", action, stack.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "stack name")
	cmd.Flags().StringVar(&description, "description", "", "optional stack description")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "labels to attach to the stack")
	cmd.Flags().StringSliceVar(&profiles, "profile", nil, "profile names to include in the stack")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite an existing stack with the same name")
	mustMarkRequired(cmd, "name", "profile")

	return cmd
}

func saveStackConfig(configPath string, cfg domain.Config, stack domain.Stack, overwrite bool) (bool, error) {
	exists := false
	for _, existing := range cfg.Stacks {
		if existing.Name == stack.Name {
			exists = true
			break
		}
	}

	if exists && !overwrite {
		return false, fmt.Errorf("stack %q already exists; use --overwrite to update it", stack.Name)
	}

	created := cfg.SetStack(stack)
	if err := storage.SaveConfig(configPath, cfg); err != nil {
		return false, fmt.Errorf("save config: %w", err)
	}

	return created, nil
}

func findStack(stacks []domain.Stack, name string) (domain.Stack, bool) {
	for _, stack := range stacks {
		if stack.Name == name {
			return stack, true
		}
	}

	return domain.Stack{}, false
}

func cloneStack(stack domain.Stack) domain.Stack {
	cloned := stack
	cloned.Labels = append([]string(nil), stack.Labels...)
	cloned.Profiles = append([]string(nil), stack.Profiles...)
	return cloned
}
