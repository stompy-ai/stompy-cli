package cmd

import (
	"fmt"

	"github.com/banton/stompy-cli/internal/output"
	"github.com/spf13/cobra"
)

// bug list/get call the dev-team bug-reports dashboard endpoint, which
// requires a separate X-API-Key header the CLI has no config surface for
// (server_hosted's normal Authorization: Bearer flow doesn't satisfy it —
// see src/api/routes/bug_reports.py::_verify_api_key upstream). Until that's
// wired up (or the backend accepts the same admin-bearer auth as its other
// admin routes), these commands will return 401 for every caller — filed as
// a follow-up to STOMPY-1462 rather than fixed here, since it's a new auth
// mode, not contract drift.
var bugCmd = &cobra.Command{
	Use:   "bug",
	Short: "View bug reports (dev team only — requires X-API-Key, not yet supported by this CLI)",
}

var bugListCmd = &cobra.Command{
	Use:   "list",
	Short: "List bug reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		resp, err := apiClient.ListBugReports(project, status, limit, offset)
		if err != nil {
			return err
		}

		f := getFormatter()
		headers := []string{"ID", "TITLE", "STATUS", "SEVERITY", "CREATED"}
		var rows [][]string
		for _, b := range resp.Reports {
			statusStr := b.Status
			severityStr := b.Severity
			if isTableOutput() {
				statusStr = output.ColorStatus(b.Status)
				severityStr = output.ColorPriority(b.Severity)
			}
			rows = append(rows, []string{
				b.ID,
				b.Title,
				statusStr,
				severityStr,
				b.CreatedAt,
			})
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nTotal: %d bug reports\n", resp.Total)
		}
		return nil
	},
}

var bugGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show bug report details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		resp, err := apiClient.GetBugReport(project, args[0])
		if err != nil {
			return err
		}

		f := getFormatter()
		fields := []output.KeyValue{
			{Key: "ID", Value: resp.ID},
			{Key: "Title", Value: resp.Title},
			{Key: "Status", Value: resp.Status},
			{Key: "Severity", Value: resp.Severity},
			{Key: "Created", Value: resp.CreatedAt},
		}
		if resp.Description != "" {
			fields = append(fields, output.KeyValue{Key: "Description", Value: resp.Description})
		}
		if resp.Steps != "" {
			fields = append(fields, output.KeyValue{Key: "Steps to Reproduce", Value: resp.Steps})
		}
		if resp.Expected != "" {
			fields = append(fields, output.KeyValue{Key: "Expected Behavior", Value: resp.Expected})
		}
		if resp.Actual != "" {
			fields = append(fields, output.KeyValue{Key: "Actual Behavior", Value: resp.Actual})
		}

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

func init() {
	bugListCmd.Flags().String("status", "", "Filter by status (new, triaged, in_progress, resolved, closed, duplicate)")
	bugListCmd.Flags().Int("limit", 0, "Limit results")
	bugListCmd.Flags().Int("offset", 0, "Offset for pagination")

	bugCmd.AddCommand(bugListCmd)
	bugCmd.AddCommand(bugGetCmd)
	rootCmd.AddCommand(bugCmd)
}
