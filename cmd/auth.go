package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
}

func init() {
	authCmd.PersistentFlags().Bool("force", false, "Force overwrite existing token file")
	authCmd.AddCommand(
		authAdminCmd,
	)
}

func initAdminApiClient() error {
	b, err := os.ReadFile("admin_token.json")
	if err != nil {
		return fmt.Errorf("Failed to open admin_token.json: %v", err)
	}
	var adminToken AdminAuthTokenResponse
	err = json.Unmarshal(b, &adminToken)
	if err != nil {
		return fmt.Errorf("Failed to parse admin_token.json: %v", err)
	}
	adminApiClient = NewAdminApiClient(adminToken.AccessToken)
	return nil
}

