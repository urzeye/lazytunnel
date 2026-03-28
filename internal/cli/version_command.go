package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/urzeye/lazytunnel/internal/buildinfo"
)

func newVersionCommand() *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if short {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), buildinfo.ShortVersion())
				return err
			}

			_, err := fmt.Fprint(cmd.OutOrStdout(), buildinfo.Details())
			return err
		},
	}

	cmd.Flags().BoolVar(&short, "short", false, "print only the version number")

	return cmd
}
