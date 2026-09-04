package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Find stale engineering resources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Scanning...")
			return err
		},
	}
}
