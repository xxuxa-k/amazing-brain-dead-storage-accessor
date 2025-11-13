package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"mime/multipart"

	"github.com/spf13/cobra"
	"github.com/xxuxa-k/amazing-brain-dead-storage-accessor/internal"
)

var as string

var loginCmd = &cobra.Command{
	Use:   "login",
	PreRunE: prerunLoginCmd,
	RunE: runLoginCmd,
}

func init() {
	loginCmd.Flags().Bool("force", false, "Force overwrite existing token file")
	loginCmd.Flags().StringVar(&as, "as", "", "Specify the user type to login (admin|user)")
	err := loginCmd.MarkFlagRequired("as")
	if err != nil {
		log.Fatalf("Failed to mark 'as' flag as required: %v", err)
	}
}

func prerunLoginCmd(cmd *cobra.Command, args []string) error {
	switch as {
	case "admin", "user":
	default:
		return fmt.Errorf("Invalid user type: %s", as)
	}
	return nil
}


func loginAsAdmin() error {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	if err := w.WriteField("code", os.Getenv("DIRECTCLOUD_CODE")); err != nil {
		return fmt.Errorf("Failed to write field 'code': %v", err)
	}
	if err := w.WriteField("service", os.Getenv("DIRECTCLOUD_ADMIN_SERVICE")); err != nil {
		return fmt.Errorf("Failed to write field 'service': %v", err)
	}
	if err := w.WriteField("service_key", os.Getenv("DIRECTCLOUD_ADMIN_SERVICE_KEY")); err != nil {
		return fmt.Errorf("Failed to write field 'service_key': %v", err)
	}
	if err := w.WriteField("id", os.Getenv("DIRECTCLOUD_ADMIN_ID")); err != nil {
		return fmt.Errorf("Failed to write field 'id': %v", err)
	}
	if err := w.WriteField("password", os.Getenv("DIRECTCLOUD_ADMIN_PASSWORD")); err != nil {
		return fmt.Errorf("Failed to write field 'password': %v", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed to close multipart writer: %v", err)
	}

	u, err := url.Parse("https://api.directcloud.jp/openapi/jauth/token")
	if err != nil {
		return fmt.Errorf("Failed to parse URL: %v", err)
	}
	params := url.Values{}
	params.Add("lang", "eng")
	u.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), &b)
	if err != nil {
		return fmt.Errorf("Failed to create login request: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to send login request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read login response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Login request failed with status: %s, body: %s", resp.Status, string(body))
	}

	token := &internal.AdminAuthTokenResponse{}
	if err := json.Unmarshal(body, token); err != nil {
		return fmt.Errorf("Failed to parse login response: %v", err)
	}
	if !token.Success || token.AccessToken == "" {
		return fmt.Errorf("Login failed: %s", string(body))
	}

	err = os.WriteFile("admin_token.json", body, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write token to file: %v", err)
	}

	fmt.Println("admin_token.json saved successfully.")
	return nil
}

func runLoginCmd(cmd *cobra.Command, args []string) error {
	if as == "admin" {
		return loginAsAdmin()
	} else if as == "user" {
		fmt.Println("login as user is not implemented yet.")
		return nil
	}
	return fmt.Errorf("Invalid user type: %s", as)
}
