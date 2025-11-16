package cmd

import (
	"net/http"
)

type AdminApiClient struct {
	AccessToken string
	httpClient  *http.Client
}
func NewAdminApiClient(token string) *AdminApiClient {
	return &AdminApiClient{
		AccessToken: token,
		httpClient: &http.Client{},
	}
}
func (c *AdminApiClient) NewGetRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("access_token", c.AccessToken)
	return req, nil
}

type AdminAuthTokenResponse struct {
	Success bool `json:"success"`
	AccessToken string `json:"access_token"`
	Expire string `json:"expire"`
	ExpireTimestamp int `json:"expire_timestamp"`
}
type AdminAuthTokenErrorResponse struct {
	Success bool `json:"success"`
	All string `json:"all"`
	ResultCode string `json:"result_code"`
}
type SharedBoxListResponse struct {
	Success bool `json:"success"`
	Total int `json:"total"`
	Lists []SharedBoxListItem `json:"lists"`
}
type SharedBoxListItem struct {
	Name string `json:"name"`
	Node string `json:"node"`
	URL string `json:"url"`
	DrivePath string `json:"drive_path"`
}

