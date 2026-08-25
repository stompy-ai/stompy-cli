package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/banton/stompy-cli/internal/config"
	"github.com/spf13/viper"
)

func TestIsExpired_FutureTime(t *testing.T) {
	// Token expiring 1 hour from a fixed reference — not expired
	futureExpiry := time.Date(2099, 1, 1, 12, 0, 0, 0, time.UTC)
	if IsExpired(futureExpiry) {
		t.Error("IsExpired() = true for far-future expiry, want false")
	}
}

func TestIsExpired_PastTime(t *testing.T) {
	// Token that expired in the past — expired
	pastExpiry := time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC)
	if !IsExpired(pastExpiry) {
		t.Error("IsExpired() = false for past expiry, want true")
	}
}

func TestIsExpired_WithinBuffer(t *testing.T) {
	// Token expiring 2 minutes from now — within 5-minute buffer — expired
	withinBuffer := time.Now().Add(2 * time.Minute)
	if !IsExpired(withinBuffer) {
		t.Error("IsExpired() = false for expiry within buffer, want true")
	}
}

func TestIsExpired_JustOutsideBuffer(t *testing.T) {
	// Token expiring 10 minutes from now — outside 5-minute buffer — not expired
	outsideBuffer := time.Now().Add(10 * time.Minute)
	if IsExpired(outsideBuffer) {
		t.Error("IsExpired() = true for expiry outside buffer, want false")
	}
}

func TestRefreshToken(t *testing.T) {
	wantToken := TokenResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			t.Errorf("ParseForm error: %v", err)
		}

		if got := r.FormValue("grant_type"); got != "refresh_token" {
			t.Errorf("grant_type = %q, want %q", got, "refresh_token")
		}
		if got := r.FormValue("refresh_token"); got != "old-refresh-token" {
			t.Errorf("refresh_token = %q, want %q", got, "old-refresh-token")
		}
		if got := r.FormValue("client_id"); got != CLIClientID {
			t.Errorf("client_id = %q, want %q", got, CLIClientID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wantToken)
	}))
	defer server.Close()

	apiURL := server.URL + "/api/v1"
	got, err := RefreshToken(apiURL, "old-refresh-token")
	if err != nil {
		t.Fatalf("RefreshToken() error: %v", err)
	}

	if got.AccessToken != wantToken.AccessToken {
		t.Errorf("AccessToken = %q, want %q", got.AccessToken, wantToken.AccessToken)
	}
	if got.RefreshToken != wantToken.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", got.RefreshToken, wantToken.RefreshToken)
	}
	if got.ExpiresIn != wantToken.ExpiresIn {
		t.Errorf("ExpiresIn = %d, want %d", got.ExpiresIn, wantToken.ExpiresIn)
	}
}

func TestRefreshToken_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := RefreshToken(server.URL+"/api/v1", "expired-refresh-token")
	if err == nil {
		t.Error("RefreshToken() expected error for 401 response, got nil")
	}
}

// resetViper clears all viper state for test isolation.
func resetViper() {
	viper.Reset()
}

func TestGetValidToken_NotLoggedIn(t *testing.T) {
	resetViper()

	_, err := GetValidToken(config.EnvProduction, "https://api.stompy.ai/api/v1")
	if err == nil {
		t.Error("GetValidToken() expected error when not logged in, got nil")
	}
}

func TestGetValidToken_ValidToken(t *testing.T) {
	resetViper()
	viper.Set("auth.production.access_token", "valid-access-token")
	viper.Set("auth.production.token_expiry", time.Now().Add(1*time.Hour).Format(time.RFC3339))

	token, err := GetValidToken(config.EnvProduction, "https://api.stompy.ai/api/v1")
	if err != nil {
		t.Fatalf("GetValidToken() error: %v", err)
	}
	if token != "valid-access-token" {
		t.Errorf("token = %q, want %q", token, "valid-access-token")
	}
}

func TestGetValidToken_ExpiredNoRefreshToken(t *testing.T) {
	resetViper()
	viper.Set("auth.production.access_token", "expired-token")
	viper.Set("auth.production.token_expiry", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	// No refresh token set

	_, err := GetValidToken(config.EnvProduction, "https://api.stompy.ai/api/v1")
	if err == nil {
		t.Error("GetValidToken() expected error when expired with no refresh token, got nil")
	}
}

func TestGetValidToken_DoesNotLeakAcrossEnvironments(t *testing.T) {
	resetViper()
	viper.Set("auth.production.access_token", "prod-token")
	viper.Set("auth.production.token_expiry", time.Now().Add(1*time.Hour).Format(time.RFC3339))
	viper.Set("auth.staging.access_token", "staging-token")
	viper.Set("auth.staging.token_expiry", time.Now().Add(1*time.Hour).Format(time.RFC3339))

	prodToken, err := GetValidToken(config.EnvProduction, "https://api.stompy.ai/api/v1")
	if err != nil {
		t.Fatalf("GetValidToken(production) error: %v", err)
	}
	if prodToken != "prod-token" {
		t.Errorf("GetValidToken(production) = %q, want %q", prodToken, "prod-token")
	}

	stagingToken, err := GetValidToken(config.EnvStaging, "https://api-staging.stompy.ai/api/v1")
	if err != nil {
		t.Fatalf("GetValidToken(staging) error: %v", err)
	}
	if stagingToken != "staging-token" {
		t.Errorf("GetValidToken(staging) = %q, want %q", stagingToken, "staging-token")
	}
}

func TestGetValidToken_RefreshesExpiredToken(t *testing.T) {
	wantToken := TokenResponse{
		AccessToken:  "refreshed-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(wantToken)
	}))
	defer server.Close()

	resetViper()
	// Isolate HOME so config.Save() doesn't overwrite real config
	t.Setenv("HOME", t.TempDir())
	viper.Set("auth.production.access_token", "expired-token")
	viper.Set("auth.production.refresh_token", "old-refresh-token")
	viper.Set("auth.production.token_expiry", time.Now().Add(-1*time.Hour).Format(time.RFC3339))

	// Use a temp dir for config save so it doesn't touch real config
	tmpDir := t.TempDir()
	viper.SetConfigFile(tmpDir + "/config.yaml")

	token, err := GetValidToken(config.EnvProduction, server.URL+"/api/v1")
	if err != nil {
		t.Fatalf("GetValidToken() error: %v", err)
	}
	if token != "refreshed-access-token" {
		t.Errorf("token = %q, want %q", token, "refreshed-access-token")
	}

	// Verify the new token was persisted in viper under the production namespace
	if got := viper.GetString("auth.production.access_token"); got != "refreshed-access-token" {
		t.Errorf("persisted access_token = %q, want %q", got, "refreshed-access-token")
	}
	// ...and did not leak into staging's namespace
	if got := viper.GetString("auth.staging.access_token"); got != "" {
		t.Errorf("refresh leaked into staging namespace: access_token = %q, want empty", got)
	}
}
