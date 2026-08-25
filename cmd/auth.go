package cmd

import (
	"fmt"
	"time"

	"github.com/banton/stompy-cli/internal/auth"
	"github.com/banton/stompy-cli/internal/config"
	"github.com/banton/stompy-cli/internal/output"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate via OAuth 2.0 browser-based login (PKCE)",
	RunE: func(cmd *cobra.Command, args []string) error {
		env := currentEnvironment()
		apiURL := resolveAPIURL()

		tokenResp, err := auth.Login(apiURL)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}

		expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
		if err := config.SaveTokens(env, tokenResp.AccessToken, tokenResp.RefreshToken, expiry, "", ""); err != nil {
			return fmt.Errorf("saving tokens: %w", err)
		}

		fmt.Printf("Login successful (%s)! Token saved to %s\n", env, config.GetConfigPath())
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored authentication tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		env := currentEnvironment()
		if err := config.ClearTokens(env); err != nil {
			return fmt.Errorf("clearing tokens: %w", err)
		}
		fmt.Printf("Logged out of %s. Tokens cleared.\n", env)
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.Load(); err != nil {
			return err
		}

		env := currentEnvironment()
		f := getFormatter()

		// Check API key first
		if flagAPIKey != "" || config.GetAPIKey() != "" {
			fmt.Print(f.FormatSingle([]output.KeyValue{
				{Key: "Environment", Value: string(env)},
				{Key: "Auth Method", Value: "API Key"},
				{Key: "Status", Value: "Authenticated"},
			}))
			return nil
		}

		// Check OAuth tokens, scoped to the target environment
		token := config.GetAccessToken(env)
		if token == "" {
			fmt.Printf("Not authenticated against %s. Run 'stompy login' to authenticate.\n", env)
			return nil
		}

		expiry := config.GetTokenExpiry(env)
		email := config.GetEmail(env)
		status := "Valid"
		if auth.IsExpired(expiry) {
			status = "Expired (will auto-refresh on next command)"
		}

		fields := []output.KeyValue{
			{Key: "Environment", Value: string(env)},
			{Key: "Auth Method", Value: "OAuth 2.0 (PKCE)"},
			{Key: "Status", Value: status},
		}
		if email != "" {
			fields = append(fields, output.KeyValue{Key: "Email", Value: email})
		}
		if !expiry.IsZero() {
			fields = append(fields, output.KeyValue{Key: "Token Expiry", Value: expiry.Local().Format(time.RFC3339)})
		}

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)
}
