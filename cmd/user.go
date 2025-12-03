package cmd


import (
	"github.com/spf13/cobra"
)

var userCmd = &cobra.Command{
	Use:   "user",
}

func init() {
	userCmd.AddCommand(
		userSyncCmd,
		)
}

