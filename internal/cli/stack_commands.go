package cli

import (
	"fmt"

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
		newStackRemoveCommand(configPath),
	)

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

			exists := false
			for _, existing := range cfg.Stacks {
				if existing.Name == stack.Name {
					exists = true
					break
				}
			}

			if exists && !overwrite {
				return fmt.Errorf("stack %q already exists; use --overwrite to update it", stack.Name)
			}

			created := cfg.SetStack(stack)
			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
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
