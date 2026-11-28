package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

// bug list/get read the canonical bug-report store over REST with the CLI's
// ordinary bearer key (STOMPY-3834): a reporter sees their own reports, staff
// see all. Proved against the real binary by dementia-production's
// test_pinned_cli_reads_own_and_refuses_other_without_new_store.
var bugCmd = &cobra.Command{
	Use:   "bug",
	Short: "View your bug reports (staff see all)",
}

var bugListCmd = &cobra.Command{
	Use:   "list",
	Short: "List bug reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Bug reports are not project-scoped; the API takes no project (STOMPY-3848).
		project := ""

		status, _ := cmd.Flags().GetString("status")
		severity, _ := cmd.Flags().GetString("severity")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		resp, err := apiClient.ListBugReportsWithSeverity(project, status, severity, limit, offset)
		if err != nil {
			return err
		}

		f := getFormatter()
		headers := []string{"ID", "TITLE", "STATUS", "SEVERITY", "CREATED", "PROJECT", "UPDATED"}
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
				b.ProjectName,
				b.UpdatedAt,
			})
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nTotal: %d bug reports\n", resp.Total)
		}
		// stderr in every format: -o json stays the row array (STOMPY-3848).
		for _, q := range resp.Quarantined {
			fmt.Fprintf(os.Stderr, "notice: %s could not be shown (%s)\n", q.ID, q.Reason)
		}
		return nil
	},
}

var bugGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show bug report details",
	Args:  exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Bug reports are not project-scoped; the API takes no project (STOMPY-3848).
		project := ""

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

		fmt.Print(formatBugDetail(f, fields, resp))
		return nil
	},
}

func init() {
	bugListCmd.Flags().String("status", "", "Filter by status (triage, confirmed, in_progress, resolved, wont_fix, parked)")
	bugListCmd.Flags().String("severity", "", "Filter by severity (critical, high, medium, low)")
	bugListCmd.Flags().Int("limit", 0, "Limit results")
	bugListCmd.Flags().Int("offset", 0, "Offset for pagination")

	bugCmd.AddCommand(bugListCmd)
	bugCmd.AddCommand(bugGetCmd)
	rootCmd.AddCommand(bugCmd)
}
