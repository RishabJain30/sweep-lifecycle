package cli

import (
	"fmt"

	"github.com/RishabJain30/sweep-lifecycle/internal/version"
	"github.com/spf13/cobra"
)

// newVersionCommand prints the same build information as `sweep --version`,
// via the same version.String() call, so the two never disagree.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the sweep version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), version.String())
			return err
		},
	}
}
