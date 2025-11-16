package cmd

import (
	"github.com/spf13/cobra"
)

var sharedboxCmd = &cobra.Command{
	Use:   "sharedbox",
}

func init() {
	sharedboxCmd.PersistentFlags().String("node", "", "node to operate on")
	sharedboxCmd.AddCommand(
		sharedboxImportCmd,
		sharedboxExportCmd,
		)
}

