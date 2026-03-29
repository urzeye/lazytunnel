package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	profileimport "github.com/urzeye/lazytunnel/internal/profileimport"
	"github.com/urzeye/lazytunnel/internal/storage"
)

func newProfileImportCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import draft profiles from existing SSH or Kubernetes config",
	}

	cmd.AddCommand(
		newProfileImportSSHConfigCommand(configPath),
		newProfileImportKubeContextsCommand(configPath),
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
			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			cfg, result, err := profileimport.ImportSSHConfig(cfg, path, overwrite)
			if err != nil {
				return err
			}

			if result.Created == 0 && result.Updated == 0 {
				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"imported SSH config from %s: 0 created, 0 updated, %d skipped\n",
					result.SourcePath,
					result.Skipped,
				)
				return nil
			}

			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(),
				"imported SSH config from %s: %d created, %d updated, %d skipped\n",
				result.SourcePath,
				result.Created,
				result.Updated,
				result.Skipped,
			)
			if len(result.ProfileNames) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "profiles: %s\n", strings.Join(result.ProfileNames, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "path to the SSH config file to import")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite existing profiles when imported names collide")

	return cmd
}

func newProfileImportKubeContextsCommand(configPath *string) *cobra.Command {
	var (
		kubeconfigPath string
		overwrite      bool
	)

	cmd := &cobra.Command{
		Use:   "kube-contexts",
		Short: "Import kubeconfig contexts as draft profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := storage.LoadConfig(*configPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			cfg, result, err := profileimport.ImportKubeContexts(cfg, kubeconfigPath, overwrite)
			if err != nil {
				return err
			}

			if result.Created == 0 && result.Updated == 0 {
				_, _ = fmt.Fprintf(
					cmd.OutOrStdout(),
					"imported kube contexts from %s: 0 created, 0 updated, %d skipped\n",
					result.SourcePath,
					result.Skipped,
				)
				return nil
			}

			if err := storage.SaveConfig(*configPath, cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			_, _ = fmt.Fprintf(
				cmd.OutOrStdout(),
				"imported kube contexts from %s: %d created, %d updated, %d skipped\n",
				result.SourcePath,
				result.Created,
				result.Updated,
				result.Skipped,
			)
			if len(result.ProfileNames) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "profiles: %s\n", strings.Join(result.ProfileNames, ", "))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "path to the kubeconfig file to import")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite existing profiles when imported names collide")

	return cmd
}
