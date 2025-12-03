package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"

	"github.com/spf13/cobra"
)

var authAdminCmd = &cobra.Command{
	Use:  "admin",
	RunE: runAuthAdminCmd,
}

func init() {}

func runAuthAdminCmd(cmd *cobra.Command, args []string) error {
	cmdCtx := cmd.Context()
	var b bytes.Buffer
	var fieldErr error
	w := multipart.NewWriter(&b)
	if err := w.WriteField("code", os.Getenv("DIRECTCLOUD_CODE")); err != nil {
		fieldErr = errors.Join(fieldErr, err)
	}
	if err := w.WriteField("service", os.Getenv("DIRECTCLOUD_ADMIN_SERVICE")); err != nil {
		fieldErr = errors.Join(fieldErr, err)
	}
	if err := w.WriteField("service_key", os.Getenv("DIRECTCLOUD_ADMIN_SERVICE_KEY")); err != nil {
		fieldErr = errors.Join(fieldErr, err)
	}
	if err := w.WriteField("id", os.Getenv("DIRECTCLOUD_ADMIN_ID")); err != nil {
		fieldErr = errors.Join(fieldErr, err)
	}
	if err := w.WriteField("password", os.Getenv("DIRECTCLOUD_ADMIN_PASSWORD")); err != nil {
		fieldErr = errors.Join(fieldErr, err)
	}
	if fieldErr != nil {
		slog.ErrorContext(cmdCtx, "Failed to write form fields", "error", fieldErr)
		return fieldErr
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("Failed to close multipart writer: %w", err)
	}

	u, err := url.Parse("https://api.directcloud.jp/openapi/jauth/token")
	if err != nil {
		return fmt.Errorf("Failed to parse URL: %w", err)
	}
	params := url.Values{}
	params.Add("lang", "eng")
	u.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), &b)
	if err != nil {
		return fmt.Errorf("Failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to send login request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read login response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Login request failed with status: %s, body: %s", resp.Status, string(body))
	}

	token := &AdminAuthTokenResponse{}
	if err := json.Unmarshal(body, token); err != nil {
		return fmt.Errorf("Failed to parse login response: %w", err)
	}
	if !token.Success || token.AccessToken == "" {
		return fmt.Errorf("Login failed: %s", string(body))
	}

	err = os.WriteFile("admin_token.json", body, 0600)
	if err != nil {
		return fmt.Errorf("Failed to write token to file: %w", err)
	}

	slog.InfoContext(cmdCtx, "admin_token.json saved successfully")
	return nil
}
