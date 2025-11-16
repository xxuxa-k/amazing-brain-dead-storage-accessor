package cmd

import "github.com/spf13/cobra"

var sharedboxExportCmd = &cobra.Command{
	Use: "export",
	RunE: runSharedboxExportCmd,
}

func init() {
}

func runSharedboxExportCmd(cmd *cobra.Command, args []string) error {
	return nil
}
