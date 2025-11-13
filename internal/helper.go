package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

func loadAdminAccessToken() (string, error) {
	b, err := os.ReadFile("admin_token.json")
	if err != nil {
		return "", fmt.Errorf("Failed to open admin_token.json: %v", err)
	}

	var adminToken AdminAuthTokenResponse
	err = json.Unmarshal(b, &adminToken)
	if err != nil {
		return "", fmt.Errorf("Failed to parse admin_token.json: %v", err)
	}
	return adminToken.AccessToken, nil
}

func NewGetRequest(url string) (*http.Request, error) {
	adminToken, err := loadAdminAccessToken()
	if err != nil {
		return nil, fmt.Errorf("Failed to load admin access token: %v", err)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create GET request: %v", err)
	}
	req.Header.Set("access_token", adminToken)
	return req, nil
}
