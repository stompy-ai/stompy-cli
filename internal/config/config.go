package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

const (
	configDirName  = ".stompy"
	configFileName = "config"
	configFileType = "yaml"

	defaultAPIURL       = "https://api.stompy.ai/api/v1"
	stagingAPIURL       = "https://api-staging.stompy.ai/api/v1"
	defaultOutputFormat = "table"
)

// Environment identifies which Stompy backend a set of stored credentials
// belongs to. Credentials are namespaced per environment (auth.<env>.*) so a
// staging login can never overwrite, or be presented as, a production token
// (STOMPY-1703).
type Environment string

const (
	EnvProduction Environment = "production"
	EnvStaging    Environment = "staging"
)

// GetConfigDir returns the path to the stompy config directory (~/.stompy).
func GetConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", configDirName)
	}
	return filepath.Join(home, configDirName)
}

// GetConfigPath returns the path to the stompy config file (~/.stompy/config.yaml).
func GetConfigPath() string {
	return filepath.Join(GetConfigDir(), configFileName+"."+configFileType)
}

// Load initializes Viper, sets defaults, and reads the config file if it exists.
func Load() error {
	viper.SetDefault("api_url", defaultAPIURL)
	viper.SetDefault("output_format", defaultOutputFormat)

	viper.SetConfigName(configFileName)
	viper.SetConfigType(configFileType)
	viper.AddConfigPath(GetConfigDir())

	viper.SetEnvPrefix("STOMPY")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		}
		return fmt.Errorf("reading config: %w", err)
	}

	if migrateLegacyAuth() {
		if err := Save(); err != nil {
			return fmt.Errorf("migrating legacy auth: %w", err)
		}
	}

	return nil
}

// migrateLegacyAuth moves a pre-STOMPY-1703 flat auth.* token — written
// before credentials were scoped per environment — into auth.production.*.
// Every login before this fix, flagged --use-staging or not, wrote to this
// same flat location and (due to a companion bug) always authenticated
// against production regardless of the flag, so production is the only
// environment a legacy token can honestly represent; treating it as staging
// would be presenting it to the wrong host, which is the exact hazard this
// migration exists to close.
//
// The legacy keys are left on disk, untouched, so a downgrade to an older
// binary still finds a token where it expects one. They are simply never
// read again by current code. Runs at most once: it no-ops as soon as
// auth.production.access_token is set, so it never clobbers a real
// production login made after migration.
func migrateLegacyAuth() bool {
	legacyToken := viper.GetString("auth.access_token")
	if legacyToken == "" {
		return false
	}
	if viper.GetString(authKey(EnvProduction, "access_token")) != "" {
		return false
	}

	viper.Set(authKey(EnvProduction, "access_token"), legacyToken)
	viper.Set(authKey(EnvProduction, "refresh_token"), viper.GetString("auth.refresh_token"))
	viper.Set(authKey(EnvProduction, "token_expiry"), viper.GetString("auth.token_expiry"))
	viper.Set(authKey(EnvProduction, "email"), viper.GetString("auth.email"))
	viper.Set(authKey(EnvProduction, "user_id"), viper.GetString("auth.user_id"))
	return true
}

// Save writes the current Viper config to the config file,
// creating the directory if needed.
func Save() error {
	dir := GetConfigDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	return viper.WriteConfigAs(GetConfigPath())
}

// GetAPIURL returns the configured API URL.
func GetAPIURL() string {
	return viper.GetString("api_url")
}

// GetStagingAPIURL returns the staging API URL.
func GetStagingAPIURL() string {
	return stagingAPIURL
}

// GetAPIKey returns the configured API key.
func GetAPIKey() string {
	return viper.GetString("api_key")
}

// GetDefaultProject returns the configured default project.
func GetDefaultProject() string {
	return viper.GetString("default_project")
}

// GetOutputFormat returns the configured output format.
func GetOutputFormat() string {
	return viper.GetString("output_format")
}

// SetValue sets a config key to the given value and saves.
func SetValue(key, value string) error {
	viper.Set(key, value)
	return Save()
}

// GetValue returns the string value for a config key.
func GetValue(key string) string {
	return viper.GetString(key)
}

// GetAllSettings returns all config settings as a map.
func GetAllSettings() map[string]any {
	return viper.AllSettings()
}

// authKey returns the namespaced viper key for an environment-scoped auth field.
func authKey(env Environment, field string) string {
	return fmt.Sprintf("auth.%s.%s", env, field)
}

// SaveTokens persists auth tokens and user info for the given environment.
// Credentials for other environments are untouched.
func SaveTokens(env Environment, accessToken, refreshToken string, expiry time.Time, email, userID string) error {
	viper.Set(authKey(env, "access_token"), accessToken)
	viper.Set(authKey(env, "refresh_token"), refreshToken)
	viper.Set(authKey(env, "token_expiry"), expiry.Format(time.RFC3339))
	viper.Set(authKey(env, "email"), email)
	viper.Set(authKey(env, "user_id"), userID)
	return Save()
}

// GetAccessToken returns the stored access token for the given environment.
func GetAccessToken(env Environment) string {
	return viper.GetString(authKey(env, "access_token"))
}

// GetRefreshToken returns the stored refresh token for the given environment.
func GetRefreshToken(env Environment) string {
	return viper.GetString(authKey(env, "refresh_token"))
}

// GetTokenExpiry returns the stored token expiry time for the given environment.
func GetTokenExpiry(env Environment) time.Time {
	s := viper.GetString(authKey(env, "token_expiry"))
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// GetEmail returns the stored user email for the given environment.
func GetEmail(env Environment) string {
	return viper.GetString(authKey(env, "email"))
}

// ClearTokens removes the stored auth tokens for the given environment and
// saves. It does not touch any other environment's credentials.
func ClearTokens(env Environment) error {
	viper.Set(authKey(env, "access_token"), "")
	viper.Set(authKey(env, "refresh_token"), "")
	viper.Set(authKey(env, "token_expiry"), "")
	viper.Set(authKey(env, "email"), "")
	viper.Set(authKey(env, "user_id"), "")
	return Save()
}

// ResolveProject determines the active project using this precedence:
// 1. Explicit flag value
// 2. STOMPY_PROJECT environment variable
// 3. default_project from config
// Returns an error if no project can be resolved.
func ResolveProject(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if env := os.Getenv("STOMPY_PROJECT"); env != "" {
		return env, nil
	}
	if dp := GetDefaultProject(); dp != "" {
		return dp, nil
	}
	return "", fmt.Errorf("no project specified. Set a default with:\n  stompy project use <name>\n\nOr pass -p <name> to any command. Run 'stompy project list' to see available projects")
}
