package internal

type AdminAuthTokenRequest struct {
	Code string `json:"code"`
	Service string `json:"service"`
	ServiceKey string `json:"service_key"`
	ID string `json:"id"`
	Password string `json:"password"`
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

