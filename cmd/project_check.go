package cmd

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/config"
	"github.com/stompy-ai/stompy-cli/internal/repomarker"
)

// checkMarker is loaded in PersistentPreRunE (before auth) so a missing or
// invalid file exits 2 without any network or credential work.
var checkMarker *repomarker.Marker

type checkRow struct {
	Address     string `json:"address"`
	Role        string `json:"role"`
	Status      string `json:"status"` // ok | not_found | unauthorized | error
	Reason      string `json:"reason,omitempty"`
	CallerRole  string `json:"caller_role,omitempty"`
	Environment string `json:"environment"`
}

var projectCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify every project in .stompy.yaml resolves for the current caller",
	Long: `For each address in the nearest .stompy.yaml, ask the API (the same call as
'stompy project info') whether it resolves for you on the file's environment.

Exit codes: 0 all resolve, 1 at least one does not, 2 missing/invalid file.`,
	Args: cobra.NoArgs,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		m, path, err := config.LoadNearestMarker()
		if err != nil {
			return &exitError{2, err.Error()}
		}
		if m == nil {
			return &exitError{2, fmt.Sprintf("no %s found in this directory or any parent. Create one with:\n  stompy init --project @owner/project", repomarker.FileName)}
		}
		if m.Environment == repomarker.EnvStaging {
			flagUseStaging = true
		} else if flagUseStaging {
			return &exitError{2, fmt.Sprintf("%s declares environment: production but --use-staging was passed; edit the file or drop the flag", path)}
		}
		checkMarker = m
		return rootCmd.PersistentPreRunE(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		m := checkMarker
		rows := make([]checkRow, 0, len(m.Projects))
		failed := false
		for _, p := range m.Projects {
			row := checkRow{Address: p.Address, Role: p.Role, Environment: m.Environment}
			resp, err := apiClient.GetProject(p.Address, false)
			switch {
			case err == nil:
				row.Status, row.CallerRole = "ok", resp.Role
			default:
				failed = true
				var apiErr *api.APIError
				if errors.As(err, &apiErr) {
					reason := apiErr.Detail
					if reason == "" {
						reason = apiErr.Message
					}
					row.Reason = reason
					switch apiErr.StatusCode {
					case http.StatusNotFound:
						row.Status = "not_found"
					case http.StatusUnauthorized, http.StatusForbidden:
						row.Status = "unauthorized"
					default:
						row.Status = "error"
						row.Reason = fmt.Sprintf("HTTP %d: %s", apiErr.StatusCode, reason)
					}
				} else {
					row.Status, row.Reason = "error", err.Error()
				}
			}
			rows = append(rows, row)
		}

		if isTableOutput() {
			for _, r := range rows {
				fmt.Println(renderCheckLine(r))
			}
		} else {
			fmt.Print(getFormatter().FormatRaw(rows))
		}
		if failed {
			return &exitError{1, ""}
		}
		return nil
	},
}

func renderCheckLine(r checkRow) string {
	label := map[string]string{"ok": "OK", "not_found": "NOT FOUND", "unauthorized": "UNAUTHORIZED", "error": "ERROR"}[r.Status]
	line := fmt.Sprintf("%-12s %s [%s, %s]", label, r.Address, r.Role, r.Environment)
	if r.Status == "ok" && r.CallerRole != "" {
		line += " (you: " + r.CallerRole + ")"
	}
	if r.Reason != "" {
		line += ": " + r.Reason
	}
	return line
}

func init() {
	projectCmd.AddCommand(projectCheckCmd)
}
