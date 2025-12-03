package cmd

import (
	"github.com/spf13/cobra"
)

var sharedboxCmd = &cobra.Command{
	Use:   "sharedbox",
}

func init() {
	sharedboxCmd.AddCommand(
		sharedboxSyncCmd,
		sharedboxExportCmd,
		)
}

