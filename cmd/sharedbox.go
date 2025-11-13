package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	sharedboxCmd.AddCommand(
		sharedboxListCmd,
		)

}

var sharedboxCmd = &cobra.Command{
	Use:   "sharedbox",
	PreRunE: runSharedboxPreRunE,
}

func runSharedboxPreRunE(cmd *cobra.Command, args []string) error {
	if mongoClient == nil {
		return fmt.Errorf("MongoDB client is not initialized")
	}
	return nil
}
