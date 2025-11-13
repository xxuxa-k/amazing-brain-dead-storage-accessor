package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var node string

var sharedboxListCmd = &cobra.Command{
	Use:   "list",
	PreRunE: prerunSharedboxListCmd,
	RunE: runSharedboxListCmd,
}

func init() {
	sharedboxListCmd.Flags().Bool("recursive", false, "List sharedboxes recursively")
	sharedboxListCmd.Flags().StringVar(&node, "node", "", "node ID to list sharedboxes from")
	err := sharedboxListCmd.MarkFlagRequired("node")
	if err != nil {
		log.Fatalf("Failed to mark 'node' flag as required: %v", err)
	}
}

func prerunSharedboxListCmd(cmd *cobra.Command, args []string) error {
	return nil
}

func runSharedboxListCmd(cmd *cobra.Command, args []string) error {
	if node == "root" {
		fmt.Println("List root sharedboxes")
	} else {
		fmt.Println("List sharedboxes for node:", node)
	}
	return nil
}
