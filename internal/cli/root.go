package cli

import "github.com/spf13/cobra"

func newRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "sweep",
		Short:         "Garbage collection for your engineering stack",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.AddCommand(newScanCommand(runScan))

	return rootCmd
}

func Execute() error {
	return newRootCommand().Execute()
}
