package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/urzeye/lazytunnel/internal/domain"
	"github.com/urzeye/lazytunnel/internal/storage"
)

func newInitCommand(configPath *string) *cobra.Command {
	var (
		force  bool
		sample bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a LazyTunnel config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				if _, err := os.Stat(*configPath); err == nil {
					return fmt.Errorf("config already exists at %q; use --force to overwrite", *configPath)
				} else if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("check existing config: %w", err)
				}
			}

			cfg := storage.SampleConfig()
			if !sample {
				cfg = domain.DefaultConfig()
			}

			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			mode := "empty"
			if sample {
				mode = "sample"
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "initialized %s config at %s\n", mode, *configPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")
	cmd.Flags().BoolVar(&sample, "sample", false, "write a sample config with example profiles and a stack")

	return cmd
}

func newValidateCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the current LazyTunnel config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(),
				"config is valid: %d profiles, %d stacks\n",
				len(cfg.Profiles),
				len(cfg.Stacks),
			)
			return nil
		},
	}
}
