package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

var conflictCmd = &cobra.Command{
	Use:   "conflict",
	Short: "Manage conflicts between contexts",
}

// validConflictStatuses is the server's actual conflict-status vocabulary
// since the 1767 A-train (STOMPY-1814 — the CLI's old documented
// vocabulary, unresolved/resolved/dismissed, had drifted from this and
// silently matched zero rows instead of erroring).
var validConflictStatuses = map[string]bool{
	"pending":       true,
	"user_resolved": true,
	"auto_resolved": true,
	"dismissed":     true,
}

func validateConflictStatus(status string) error {
	if status == "" || validConflictStatuses[strings.ToLower(status)] {
		return nil
	}
	return fmt.Errorf(
		"invalid status %q: must be one of pending, user_resolved, auto_resolved, dismissed",
		status,
	)
}

var conflictListCmd = &cobra.Command{
	Use:   "list",
	Short: "List detected conflicts",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		if err := validateConflictStatus(status); err != nil {
			return err
		}

		resp, err := apiClient.ListConflicts(project, status, limit, offset)
		if err != nil {
			return err
		}

		f := getFormatter()
		headers := []string{"ID", "ITEM A", "ITEM B", "TYPE", "CONFIDENCE", "STATUS"}
		var rows [][]string
		for _, c := range resp.Conflicts {
			row := []string{
				c.ID,
				fmt.Sprintf("%s #%d", c.ItemA.ItemType, c.ItemA.ItemID),
				fmt.Sprintf("%s #%d", c.ItemB.ItemType, c.ItemB.ItemID),
				c.ContradictionType,
				fmt.Sprintf("%.2f", c.Confidence),
			}
			if isTableOutput() {
				row = append(row, output.ColorStatus(c.Status))
			} else {
				row = append(row, c.Status)
			}
			rows = append(rows, row)
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nTotal: %d conflicts (%d pending, %d resolved)\n", resp.Total, resp.PendingCount, resp.ResolvedCount)
		}
		return nil
	},
}

var conflictGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show conflict details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		resp, err := apiClient.GetConflict(project, args[0])
		if err != nil {
			return err
		}

		f := getFormatter()
		fields := []output.KeyValue{
			{Key: "ID", Value: resp.ID},
			{Key: "Item A", Value: fmt.Sprintf("%s #%d: %s", resp.ItemA.ItemType, resp.ItemA.ItemID, resp.ItemA.Excerpt)},
			{Key: "Item B", Value: fmt.Sprintf("%s #%d: %s", resp.ItemB.ItemType, resp.ItemB.ItemID, resp.ItemB.Excerpt)},
			{Key: "Type", Value: resp.ContradictionType},
			{Key: "Confidence", Value: fmt.Sprintf("%.2f", resp.Confidence)},
			{Key: "Status", Value: resp.Status},
		}
		if resp.CreatedAt != nil {
			fields = append(fields, output.KeyValue{Key: "Created", Value: formatTimestamp(*resp.CreatedAt)})
		}
		if resp.Resolution != nil {
			fields = append(fields, output.KeyValue{Key: "Resolution", Value: *resp.Resolution})
		}
		if resp.ResolutionNotes != nil {
			fields = append(fields, output.KeyValue{Key: "Resolution Notes", Value: *resp.ResolutionNotes})
		}
		if resp.ResolvedAt != nil {
			fields = append(fields, output.KeyValue{Key: "Resolved At", Value: formatTimestamp(*resp.ResolvedAt)})
		}

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

var conflictDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Trigger conflict detection",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		scope, _ := cmd.Flags().GetString("scope")
		req := api.ConflictDetectRequest{Scope: scope}

		resp, queued, err := apiClient.DetectConflicts(project, req)
		if err != nil {
			return err
		}

		if queued != nil {
			// STOMPY-1389: scope=all is queued as a background job rather
			// than blocking (a full scan can run minutes) — poll `conflict
			// list` for results.
			fmt.Printf("%s %s (job %s)\n", output.Success("⧗"), queued.Message, queued.JobID)
			return nil
		}

		fmt.Printf("%s Found %d conflicts (%d auto-resolved, %d pending) in %.0fms\n",
			output.Success("✓"),
			resp.ConflictsFound,
			resp.AutoResolved,
			resp.Pending,
			resp.ProcessingTimeMs)
		return nil
	},
}

var conflictResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "Resolve a conflict",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		resolution, _ := cmd.Flags().GetString("resolution")
		if resolution == "" {
			return fmt.Errorf("--resolution is required (dismiss, keep_a, keep_b, merge)")
		}

		valid := map[string]bool{"dismiss": true, "keep_a": true, "keep_b": true, "merge": true}
		if !valid[strings.ToLower(resolution)] {
			return fmt.Errorf("invalid resolution %q: must be one of dismiss, keep_a, keep_b, merge", resolution)
		}

		req := api.ConflictResolveRequest{Resolution: resolution}
		resp, err := apiClient.ResolveConflict(project, args[0], req)
		if err != nil {
			return err
		}

		fmt.Print(resolveSummary(resp, project))
		return nil
	},
}

// resolveSummary is what `conflict resolve` prints. keep_a / keep_b ARCHIVE
// the rejected side's topic server-side (STOMPY-2663), so the summary names it
// and the one-line undo — a bare "resolved" would hide that a context just
// left recall. dismiss and merge print exactly the line they always did.
//
// The undo is the CLI's OWN command, never the server's `undo` string: that
// text is worded for the door that asked (MCP set_lifecycle, REST route), and
// one string cannot be right for every door.
func resolveSummary(resp *api.ConflictResponse, project string) string {
	line := fmt.Sprintf("%s Conflict %s resolved (%s)\n", output.Success("✓"), resp.ID, resp.Status)
	a := resp.ArchivedContext
	if a == nil {
		return line
	}
	if !a.Archived {
		return line + fmt.Sprintf("  nothing archived (%s)\n", a.Reason)
	}
	undo := fmt.Sprintf("stompy context lifecycle %s -p %s --state active", a.Topic, project)
	return line + fmt.Sprintf("  archived %s v%s (kept %s); undo: %s\n", a.Topic, a.Version, a.KeptTopic, undo)
}

func init() {
	conflictListCmd.Flags().String("status", "", "Filter by status (pending, auto_resolved, user_resolved, dismissed)")
	conflictListCmd.Flags().Int("limit", 0, "Limit results")
	conflictListCmd.Flags().Int("offset", 0, "Offset for pagination")

	conflictDetectCmd.Flags().String("scope", "", "Detection scope (all, recent)")

	conflictResolveCmd.Flags().String("resolution", "", "Resolution (required): keep_a / keep_b keep that side and ARCHIVE the other side's topic (undo: stompy context lifecycle <topic> --state active); dismiss = not a conflict, merge: nothing is archived")

	conflictCmd.AddCommand(conflictListCmd)
	conflictCmd.AddCommand(conflictGetCmd)
	conflictCmd.AddCommand(conflictDetectCmd)
	conflictCmd.AddCommand(conflictResolveCmd)
	rootCmd.AddCommand(conflictCmd)
}
