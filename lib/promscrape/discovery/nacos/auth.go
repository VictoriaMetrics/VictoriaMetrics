package nacos

import (
	"encoding/json"
	"fmt"
	"time"
)

type authResponse struct {
	AccessToken string `json:"accessToken"`
	TokenTtl    int64  `json:"tokenTtl"`
	GlobalAdmin bool   `json:"globalAdmin"`
	Username    string `json:"username"`
}

type authToken struct {
	AccessToken string    `json:"accessToken"`
	ExpireAt    time.Time `json:"expireAt"`
}

// parseAuthResponse
func parseAuthResponse(data []byte) (*authResponse, error) {
	var authResp authResponse
	if err := json.Unmarshal(data, &authResp); err != nil {
		return nil, fmt.Errorf("cannot parse auth response: %w", err)
	}
	return &authResp, nil
}
