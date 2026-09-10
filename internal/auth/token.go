package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/banton/stompy-cli/internal/config"
)

const TokenExpiryBuffer = 5 * time.Minute

// IsExpired checks if a token expiry time has passed (with 5-minute safety buffer).
func IsExpired(expiry time.Time) bool {
	return time.Now().After(expiry.Add(-TokenExpiryBuffer))
}

// RefreshToken uses a refresh token to obtain a new access token.
func RefreshToken(apiURL, refreshToken string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {CLIClientID},
	}

	tokenURL := strings.TrimSuffix(apiURL, "/api/v1") + "/oauth/token"
	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (status %d) — please run 'stompy login' again", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("decoding refresh response: %w", err)
	}

	return &tokenResp, nil
}

// GetValidToken returns a valid access token for the given environment. It
// checks the stored token's expiry, refreshes if needed, persists updated
// tokens under that same environment, and returns the access token string.
// Returns an error if no token is stored or refresh fails.
func GetValidToken(env config.Environment, apiURL string) (string, error) {
	accessToken := config.GetAccessToken(env)
	if accessToken == "" {
		return "", fmt.Errorf("not logged in — please run 'stompy login'")
	}

	expiry := config.GetTokenExpiry(env)
	if !IsExpired(expiry) {
		return accessToken, nil
	}

	// Token expired or within buffer — try refresh
	rt := config.GetRefreshToken(env)
	if rt == "" {
		return "", fmt.Errorf("token expired and no refresh token available — please run 'stompy login'")
	}

	tokenResp, err := RefreshToken(apiURL, rt)
	if err != nil {
		return "", err
	}

	newExpiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if err := config.SaveTokens(env, tokenResp.AccessToken, tokenResp.RefreshToken, newExpiry, config.GetEmail(env), ""); err != nil {
		return "", fmt.Errorf("saving refreshed tokens: %w", err)
	}

	return tokenResp.AccessToken, nil
}
