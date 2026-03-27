package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/urzeye/lazytunnel/internal/storage"
	"github.com/urzeye/lazytunnel/internal/tui"
)

func NewRootCommand() *cobra.Command {
	configPath := storage.DefaultConfigPath()

	cmd := &cobra.Command{
		Use:           "lazytunnel",
		Short:         "Manage SSH tunnels and Kubernetes port-forwards from one terminal UI",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := storage.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("load config %q: %w", configPath, err)
			}

			program := tea.NewProgram(
				tui.NewModel(cfg, configPath),
				tea.WithAltScreen(),
			)

			if _, err := program.Run(); err != nil {
				return fmt.Errorf("run terminal UI: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(
		&configPath,
		"config",
		configPath,
		"path to the LazyTunnel config file",
	)

	return cmd
}
