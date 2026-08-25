package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/banton/stompy-cli/internal/auth"
	"github.com/banton/stompy-cli/internal/config"
	"github.com/banton/stompy-cli/internal/output"
	"github.com/banton/stompy-cli/internal/update"
	"github.com/spf13/cobra"
)

var (
	flagAPIURL     string
	flagAPIKey     string
	flagProject    string
	flagOutput     string
	flagVerbose    bool
	flagUseStaging bool

	apiClient        *api.Client
	mcpClient        *api.MCPClient
	updateAvailable  = make(chan string, 1)
)

var rootCmd = &cobra.Command{
	Use:           "stompy",
	Short:         "Stompy CLI — manage projects, contexts, and tickets",
	Long:          `A command-line interface for the Stompy API. Manage projects, contexts, and tickets from your terminal.`,
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Fire off async version check (non-blocking, result printed in PostRun)
		go func() {
			if latest := update.CheckForUpdate(Version, config.GetConfigDir()); latest != "" {
				updateAvailable <- latest
			}
			close(updateAvailable)
		}()

		if !commandNeedsAuth(cmd.CommandPath(), cmd.Name(), cmd.HasParent(), cmd.HasSubCommands() && len(args) == 0) {
			return config.Load()
		}

		if err := config.Load(); err != nil {
			return err
		}

		token, err := resolveAuthToken()
		if err != nil {
			return err
		}

		apiURL := resolveAPIURL()

		apiClient = api.NewClient(apiURL, token, Version, flagVerbose)
		mcpClient = api.NewMCPClient(api.MCPBaseURL(apiURL), token, Version, flagVerbose)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagAPIURL, "api-url", "", "Override API base URL")
	rootCmd.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "Override API key")
	rootCmd.PersistentFlags().StringVarP(&flagProject, "project", "p", "", "Override default project")
	rootCmd.PersistentFlags().StringVarP(&flagOutput, "output", "o", "", "Output format: table, json, yaml")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Debug HTTP logging")
	rootCmd.PersistentFlags().BoolVar(&flagUseStaging, "use-staging", false, "")
	rootCmd.PersistentFlags().MarkHidden("use-staging")
}

// Execute is the main entry point for the CLI.
func Execute() {
	err := rootCmd.Execute()

	// Print update notice (if available) after command output
	select {
	case latest := <-updateAvailable:
		if latest != "" && isTableOutput() {
			fmt.Fprintf(os.Stderr, "\n%s A new version of stompy is available (%s). Run %s to upgrade.\n",
				output.Dim("→"),
				output.Teal(latest),
				output.Teal("stompy update"))
		}
	default:
		// Check didn't complete in time — skip silently
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, output.Error("Error:")+"\n  "+err.Error())
		os.Exit(1)
	}
}

// commandNeedsAuth reports whether a command needs config + auth resolution
// before it runs. login/logout/whoami/version/update, shell completion, config
// subcommands, and parent/grouping commands manage their own auth state (or
// none at all) and must always reach their own RunE — in particular, whoami
// has to run even with no stored token so it can report which environment
// it checked, instead of failing inside PersistentPreRunE with a generic
// "not authenticated" error that never names an environment.
//
// cmdPath is matched in full (not cmd.Name()) to avoid collisions between a
// top-level command and a subcommand that shares its name (e.g. "stompy
// update" vs "stompy context update").
func commandNeedsAuth(cmdPath, cmdName string, hasParent, isParentGrouping bool) bool {
	switch cmdPath {
	case "stompy login", "stompy logout", "stompy whoami", "stompy version", "stompy update":
		return false
	}
	switch cmdName {
	case "completion", "bash", "zsh", "fish", "powershell":
		return false
	}
	if strings.Contains(cmdPath, "config ") {
		return false
	}
	if !hasParent || isParentGrouping {
		return false
	}
	return true
}

// currentEnvironment returns which backend environment this invocation
// targets, based on --use-staging. Auth tokens are stored and read scoped to
// this environment (STOMPY-1703) so a staging session can never collide with
// a production one, or vice versa.
func currentEnvironment() config.Environment {
	if flagUseStaging {
		return config.EnvStaging
	}
	return config.EnvProduction
}

// resolveAPIURL returns the API base URL for the current invocation:
// --api-url override, then --use-staging, then the default production URL.
func resolveAPIURL() string {
	if flagAPIURL != "" {
		return flagAPIURL
	}
	if flagUseStaging {
		return config.GetStagingAPIURL()
	}
	return config.GetAPIURL()
}

// resolveAuthToken determines the auth token using precedence:
// --api-key flag > STOMPY_API_KEY env > OAuth token (with auto-refresh) > api_key from config > error
func resolveAuthToken() (string, error) {
	// 1. --api-key flag
	if flagAPIKey != "" {
		return flagAPIKey, nil
	}

	// 2. STOMPY_API_KEY env var
	if envKey := os.Getenv("STOMPY_API_KEY"); envKey != "" {
		return envKey, nil
	}

	// 3. OAuth token from config (with auto-refresh), scoped to the target environment
	env := currentEnvironment()
	accessToken := config.GetAccessToken(env)
	if accessToken != "" {
		expiry := config.GetTokenExpiry(env)
		if !auth.IsExpired(expiry) {
			return accessToken, nil
		}

		// Try to refresh
		refreshToken := config.GetRefreshToken(env)
		if refreshToken != "" {
			tokenResp, err := auth.RefreshToken(resolveAPIURL(), refreshToken)
			if err == nil {
				newExpiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
				_ = config.SaveTokens(env, tokenResp.AccessToken, tokenResp.RefreshToken, newExpiry, config.GetEmail(env), "")
				return tokenResp.AccessToken, nil
			}
			// Refresh failed — fall through
		}
	}

	// 4. Static api_key from config
	if apiKey := config.GetAPIKey(); apiKey != "" {
		return apiKey, nil
	}

	// 5. No auth available
	return "", fmt.Errorf("not authenticated. Run 'stompy login' or set STOMPY_API_KEY")
}

// getProject resolves the active project name.
func getProject() (string, error) {
	return config.ResolveProject(flagProject)
}

// getFormatter returns the output formatter based on flags and config.
func getFormatter() output.Formatter {
	return output.NewFormatter(getOutputFormat())
}

// getOutputFormat returns the resolved output format string.
func getOutputFormat() string {
	if flagOutput != "" {
		return flagOutput
	}
	return config.GetOutputFormat()
}

// isTableOutput returns true when output format is table (colors allowed).
func isTableOutput() bool {
	f := getOutputFormat()
	return f == "" || f == "table"
}
