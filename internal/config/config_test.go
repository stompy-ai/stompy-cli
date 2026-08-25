package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// setupTestConfig creates a temp dir, points viper at it, and returns a cleanup func.
func setupTestConfig(t *testing.T) string {
	t.Helper()
	viper.Reset()

	tmpDir := t.TempDir()
	// Override home so GetConfigDir uses our temp dir
	t.Setenv("HOME", tmpDir)

	return tmpDir
}

func TestLoadDefaults(t *testing.T) {
	setupTestConfig(t)

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got := GetAPIURL(); got != defaultAPIURL {
		t.Errorf("GetAPIURL() = %q, want %q", got, defaultAPIURL)
	}
	if got := GetOutputFormat(); got != defaultOutputFormat {
		t.Errorf("GetOutputFormat() = %q, want %q", got, defaultOutputFormat)
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := setupTestConfig(t)

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	viper.Set("api_key", "test-key-123")
	viper.Set("default_project", "my-project")

	if err := Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file was written
	configPath := filepath.Join(tmpDir, configDirName, configFileName+"."+configFileType)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("config file not created at %s", configPath)
	}

	// Reset and re-load
	viper.Reset()
	if err := Load(); err != nil {
		t.Fatalf("Load() after save error: %v", err)
	}

	if got := GetAPIKey(); got != "test-key-123" {
		t.Errorf("GetAPIKey() = %q, want %q", got, "test-key-123")
	}
	if got := GetDefaultProject(); got != "my-project" {
		t.Errorf("GetDefaultProject() = %q, want %q", got, "my-project")
	}
}

func TestSetValueAndGetValue(t *testing.T) {
	setupTestConfig(t)

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if err := SetValue("api_key", "new-key"); err != nil {
		t.Fatalf("SetValue() error: %v", err)
	}

	if got := GetValue("api_key"); got != "new-key" {
		t.Errorf("GetValue(api_key) = %q, want %q", got, "new-key")
	}
}

func TestGetAllSettings(t *testing.T) {
	setupTestConfig(t)

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	settings := GetAllSettings()
	if _, ok := settings["api_url"]; !ok {
		t.Error("GetAllSettings() missing api_url key")
	}
	if _, ok := settings["output_format"]; !ok {
		t.Error("GetAllSettings() missing output_format key")
	}
}

func TestSaveAndClearTokens(t *testing.T) {
	setupTestConfig(t)

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	fixedTime := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	if err := SaveTokens(EnvProduction, "access-tok", "refresh-tok", fixedTime, "user@test.com", "user-123"); err != nil {
		t.Fatalf("SaveTokens() error: %v", err)
	}

	if got := GetAccessToken(EnvProduction); got != "access-tok" {
		t.Errorf("GetAccessToken(production) = %q, want %q", got, "access-tok")
	}
	if got := GetRefreshToken(EnvProduction); got != "refresh-tok" {
		t.Errorf("GetRefreshToken(production) = %q, want %q", got, "refresh-tok")
	}
	if got := GetTokenExpiry(EnvProduction); !got.Equal(fixedTime) {
		t.Errorf("GetTokenExpiry(production) = %v, want %v", got, fixedTime)
	}
	if got := GetEmail(EnvProduction); got != "user@test.com" {
		t.Errorf("GetEmail(production) = %q, want %q", got, "user@test.com")
	}

	if err := ClearTokens(EnvProduction); err != nil {
		t.Fatalf("ClearTokens() error: %v", err)
	}

	if got := GetAccessToken(EnvProduction); got != "" {
		t.Errorf("after ClearTokens(), GetAccessToken(production) = %q, want empty", got)
	}
	if got := GetRefreshToken(EnvProduction); got != "" {
		t.Errorf("after ClearTokens(), GetRefreshToken(production) = %q, want empty", got)
	}
	if got := GetTokenExpiry(EnvProduction); !got.IsZero() {
		t.Errorf("after ClearTokens(), GetTokenExpiry(production) = %v, want zero", got)
	}
}

func TestTokensAreScopedPerEnvironment(t *testing.T) {
	setupTestConfig(t)
	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	prodExpiry := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stagingExpiry := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	if err := SaveTokens(EnvProduction, "prod-access", "prod-refresh", prodExpiry, "prod@test.com", "prod-user"); err != nil {
		t.Fatalf("SaveTokens(production) error: %v", err)
	}
	if err := SaveTokens(EnvStaging, "staging-access", "staging-refresh", stagingExpiry, "staging@test.com", "staging-user"); err != nil {
		t.Fatalf("SaveTokens(staging) error: %v", err)
	}

	// The core STOMPY-1703 regression: production and staging tokens must
	// never read back as identical, or as each other's.
	if got := GetAccessToken(EnvProduction); got != "prod-access" {
		t.Errorf("GetAccessToken(production) = %q, want %q", got, "prod-access")
	}
	if got := GetAccessToken(EnvStaging); got != "staging-access" {
		t.Errorf("GetAccessToken(staging) = %q, want %q", got, "staging-access")
	}
	if got := GetEmail(EnvProduction); got != "prod@test.com" {
		t.Errorf("GetEmail(production) = %q, want %q", got, "prod@test.com")
	}
	if got := GetEmail(EnvStaging); got != "staging@test.com" {
		t.Errorf("GetEmail(staging) = %q, want %q", got, "staging@test.com")
	}
	if got := GetTokenExpiry(EnvProduction); !got.Equal(prodExpiry) {
		t.Errorf("GetTokenExpiry(production) = %v, want %v", got, prodExpiry)
	}
	if got := GetTokenExpiry(EnvStaging); !got.Equal(stagingExpiry) {
		t.Errorf("GetTokenExpiry(staging) = %v, want %v", got, stagingExpiry)
	}

	// Clearing one environment must not touch the other.
	if err := ClearTokens(EnvStaging); err != nil {
		t.Fatalf("ClearTokens(staging) error: %v", err)
	}
	if got := GetAccessToken(EnvStaging); got != "" {
		t.Errorf("after ClearTokens(staging), GetAccessToken(staging) = %q, want empty", got)
	}
	if got := GetAccessToken(EnvProduction); got != "prod-access" {
		t.Errorf("ClearTokens(staging) leaked into production: GetAccessToken(production) = %q, want %q", got, "prod-access")
	}
}

func TestLoadMigratesLegacyAuthToProduction(t *testing.T) {
	tmpDir := setupTestConfig(t)

	// Simulate a pre-STOMPY-1703 config file on disk: flat auth.* keys with
	// no environment scoping, exactly what every CLI version before this fix wrote.
	dir := filepath.Join(tmpDir, configDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	legacyYAML := `api_url: https://api.stompy.ai/api/v1
auth:
  access_token: legacy-access
  refresh_token: legacy-refresh
  token_expiry: "2026-01-15T12:00:00Z"
  email: legacy@test.com
  user_id: legacy-user
`
	path := filepath.Join(dir, configFileName+"."+configFileType)
	if err := os.WriteFile(path, []byte(legacyYAML), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// A user with an existing token must not be silently logged out: the
	// legacy token must now read back under production, unchanged.
	if got := GetAccessToken(EnvProduction); got != "legacy-access" {
		t.Errorf("GetAccessToken(production) = %q, want %q (migrated)", got, "legacy-access")
	}
	if got := GetRefreshToken(EnvProduction); got != "legacy-refresh" {
		t.Errorf("GetRefreshToken(production) = %q, want %q (migrated)", got, "legacy-refresh")
	}
	if got := GetEmail(EnvProduction); got != "legacy@test.com" {
		t.Errorf("GetEmail(production) = %q, want %q (migrated)", got, "legacy@test.com")
	}

	// A legacy token must never be silently treated as staging's — it can
	// only have been a production token (that was the only place a token
	// could reach before this fix).
	if got := GetAccessToken(EnvStaging); got != "" {
		t.Errorf("GetAccessToken(staging) = %q, want empty — legacy token must not be presented as staging's", got)
	}

	// The legacy flat keys must remain on disk, untouched, for downgrade safety.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if !strings.Contains(string(raw), "legacy-access") {
		t.Error("expected legacy access_token to remain on disk after migration, for downgrade safety")
	}
}

func TestLoadMigrationIsIdempotent(t *testing.T) {
	tmpDir := setupTestConfig(t)

	dir := filepath.Join(tmpDir, configDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	legacyYAML := "auth:\n  access_token: legacy-access\n"
	path := filepath.Join(dir, configFileName+"."+configFileType)
	if err := os.WriteFile(path, []byte(legacyYAML), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// A real production login after migration must win — the migration must
	// never re-run and clobber it back to the stale legacy value.
	if err := SaveTokens(EnvProduction, "fresh-access", "fresh-refresh", time.Now(), "user@test.com", "u1"); err != nil {
		t.Fatalf("SaveTokens() error: %v", err)
	}

	viper.Reset()
	if err := Load(); err != nil {
		t.Fatalf("second Load() error: %v", err)
	}
	if got := GetAccessToken(EnvProduction); got != "fresh-access" {
		t.Errorf("GetAccessToken(production) = %q, want %q (migration must not re-clobber a real login)", got, "fresh-access")
	}
}

func TestLoadWithoutLegacyAuthDoesNotMigrateOrWrite(t *testing.T) {
	tmpDir := setupTestConfig(t)

	dir := filepath.Join(tmpDir, configDirName)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	// A config file with no auth section at all (e.g. user only set a default project).
	path := filepath.Join(dir, configFileName+"."+configFileType)
	if err := os.WriteFile(path, []byte("default_project: my-project\n"), 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}

	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if got := GetAccessToken(EnvProduction); got != "" {
		t.Errorf("GetAccessToken(production) = %q, want empty — nothing to migrate", got)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Error("Load() rewrote the config file even though there was no legacy auth to migrate")
	}
}

func TestResolveProject_FlagValue(t *testing.T) {
	setupTestConfig(t)
	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	got, err := ResolveProject("flag-project")
	if err != nil {
		t.Fatalf("ResolveProject() error: %v", err)
	}
	if got != "flag-project" {
		t.Errorf("ResolveProject() = %q, want %q", got, "flag-project")
	}
}

func TestResolveProject_EnvVar(t *testing.T) {
	setupTestConfig(t)
	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	t.Setenv("STOMPY_PROJECT", "env-project")

	got, err := ResolveProject("")
	if err != nil {
		t.Fatalf("ResolveProject() error: %v", err)
	}
	if got != "env-project" {
		t.Errorf("ResolveProject() = %q, want %q", got, "env-project")
	}
}

func TestResolveProject_DefaultConfig(t *testing.T) {
	setupTestConfig(t)
	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	viper.Set("default_project", "config-project")

	got, err := ResolveProject("")
	if err != nil {
		t.Fatalf("ResolveProject() error: %v", err)
	}
	if got != "config-project" {
		t.Errorf("ResolveProject() = %q, want %q", got, "config-project")
	}
}

func TestResolveProject_Error(t *testing.T) {
	setupTestConfig(t)
	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	_, err := ResolveProject("")
	if err == nil {
		t.Error("ResolveProject() expected error when no project available, got nil")
	}
}

func TestResolveProject_Precedence(t *testing.T) {
	setupTestConfig(t)
	if err := Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Set all three sources
	viper.Set("default_project", "config-project")
	t.Setenv("STOMPY_PROJECT", "env-project")

	// Flag takes precedence
	got, err := ResolveProject("flag-project")
	if err != nil {
		t.Fatalf("ResolveProject() error: %v", err)
	}
	if got != "flag-project" {
		t.Errorf("ResolveProject() = %q, want %q (flag should win)", got, "flag-project")
	}

	// Without flag, env takes precedence over config
	got, err = ResolveProject("")
	if err != nil {
		t.Fatalf("ResolveProject() error: %v", err)
	}
	if got != "env-project" {
		t.Errorf("ResolveProject() = %q, want %q (env should win over config)", got, "env-project")
	}
}
