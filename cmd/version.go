package cmd

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/banton/stompy-cli/internal/config"
	"github.com/banton/stompy-cli/internal/output"
	"github.com/banton/stompy-cli/internal/update"
	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the stompy CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("stompy-cli %s\n", Version)

		// Try to fetch API version from server
		apiURL := resolveAPIURL()
		if apiURL != "" {
			c := api.NewClient(apiURL, "", Version, false)
			// Ping health endpoint to get version headers
			_, _, err := c.Do(http.MethodGet, "/health", nil, url.Values{})
			if err == nil && c.APIVersion != "" {
				fmt.Printf("API: %s (server %s)\n", apiURL, c.APIVersion)
			} else {
				fmt.Printf("API: %s\n", apiURL)
			}
		}

		// Check for newer version
		if latest := update.CheckForUpdate(Version, config.GetConfigDir()); latest != "" {
			fmt.Printf("%s available: %s (run %s to upgrade)\n",
				output.Dim("Update"),
				output.Teal(latest),
				output.Teal("stompy update"))
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
