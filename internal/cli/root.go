package cli

import (
	"github.com/RishabJain30/sweep-lifecycle/internal/version"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "sweep",
		Short:         "Garbage collection for your engineering stack",
		Version:       version.String(),
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.AddCommand(newScanCommand(runScan))
	rootCmd.AddCommand(newExplainCommand(runExplain))
	rootCmd.AddCommand(newVersionCommand())

	return rootCmd
}

func Execute() error {
	return newRootCommand().Execute()
}
