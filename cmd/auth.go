package cmd

import (
	"fmt"
	"time"

	"github.com/banton/stompy-cli/internal/api"
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
			token := flagAPIKey
			if token == "" {
				token = config.GetAPIKey()
			}
			fields := []output.KeyValue{
				{Key: "Environment", Value: string(env)},
				{Key: "Auth Method", Value: "API Key"},
				{Key: "Status", Value: "Authenticated"},
			}
			fmt.Print(f.FormatSingle(append(fields, usageFields(token)...)))
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
		fields = append(fields, usageFields(token)...)

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

// usageFields fetches GET /billing/usage (STOMPY-2187) for `whoami`
// (STOMPY-2232 item 2) and renders it as key/value rows: Tier, Units Used
// (used/cap, or the unavailable reason when the meter can't measure — never
// a fabricated 0), and Resets (only for a windowed, non-lifetime cap).
// Usage is best-effort: a fetch error (old server, network) leaves whoami's
// auth-status answer intact rather than failing the command over it.
func usageFields(token string) []output.KeyValue {
	client := api.NewClient(resolveAPIURL(), token, Version, flagVerbose)
	usage, err := client.GetUsage()
	if err != nil || usage == nil {
		return nil
	}
	fields := []output.KeyValue{{Key: "Tier", Value: usage.Tier}}
	switch {
	case usage.UnavailableReason != nil && *usage.UnavailableReason != "":
		fields = append(fields, output.KeyValue{Key: "Units Used", Value: "unavailable (" + *usage.UnavailableReason + ")"})
	case usage.UnitsUsed != nil && usage.Cap != nil:
		fields = append(fields, output.KeyValue{
			Key:   "Units Used",
			Value: fmt.Sprintf("%d / %d", *usage.UnitsUsed, *usage.Cap),
		})
		if usage.PeriodEnd != nil && *usage.PeriodEnd != "" {
			fields = append(fields, output.KeyValue{Key: "Resets", Value: *usage.PeriodEnd})
		}
	}
	return fields
}

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)
}
