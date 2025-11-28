package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
func (c *AdminApiClient) SharedboxesList(
	ctx context.Context,
	node string,
) (SharedBoxListResponse, error) {
	var result SharedBoxListResponse
	baseURL := "https://api.directcloud.jp/openapp/m1/sharedboxes/lists/"
	joined, err := url.JoinPath(baseURL, node)
	if err != nil {
		return result, fmt.Errorf("Failed to join URL path: %w", err)
	}
	u, err := url.Parse(joined)
	if err != nil {
		return result, fmt.Errorf("Failed to parse URL: %w", err)
	}
	params := url.Values{}
	params.Add("lang", "eng")
	u.RawQuery = params.Encode()
	req, err := adminApiClient.NewGetRequest(u.String())
	if err != nil {
		return result, fmt.Errorf("Failed to create GET request: %w", err)
	}
	req = req.WithContext(ctx)
	resp, err := adminApiClient.httpClient.Do(req)
	if err != nil {
		return result, fmt.Errorf("Failed to send GET request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, fmt.Errorf("Failed to read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return result, fmt.Errorf("Failed to parse response JSON: %w", err)
	}
	return result, nil
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
	Name string `json:"name" bson:"name"`
	Node string `json:"node" bson:"node"`
	URL string `json:"url" bson:"url"`
	DrivePath string `json:"drive_path" bson:"drive_path"`
}

type SharedBoxListItemWithParent struct {
	Item SharedBoxListItem `json:"item" bson:"item"`
	ParentNode string `json:"parent_node" bson:"parent_node"`
}

type NodeError struct {
	Node string
	Err error
}
func (e *NodeError) Error() string {
	return fmt.Sprintf("NodeError: node=%s, err=%v", e.Node, e.Err)
}
