package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	sharedboxCmd.AddCommand(
		sharedboxListCmd,
		)

}

var sharedboxCmd = &cobra.Command{
	Use:   "sharedbox",
	// RunE: runSharedboxCmd,
}

// func runSharedboxCmd(cmd *cobra.Command, args []string) error {
// 	return nil
// }
