package cmd

import (
	"errors"
	"fmt"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/banton/stompy-cli/internal/config"
	"github.com/banton/stompy-cli/internal/output"
	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
}

var projectCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc, _ := cmd.Flags().GetString("description")
		req := api.ProjectCreate{Name: args[0]}
		if desc != "" {
			req.Description = &desc
		}

		resp, err := apiClient.CreateProject(req)
		if err != nil {
			return err
		}

		f := getFormatter()
		fmt.Print(f.FormatSingle([]output.KeyValue{
			{Key: "Name", Value: resp.Name},
			{Key: "Schema", Value: resp.SchemaName},
			{Key: "Created", Value: resp.CreatedAt.Local().Format("2006-01-02 15:04:05")},
		}))
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		withStats, _ := cmd.Flags().GetBool("stats")

		resp, err := apiClient.ListProjects(withStats)
		if err != nil {
			return err
		}

		f := getFormatter()
		headers := []string{"NAME", "SCHEMA", "CREATED", "ROLE"}
		if withStats {
			headers = append(headers, "CONTEXTS", "SESSIONS", "FILES")
		}

		var rows [][]string
		for _, p := range resp.Projects {
			row := []string{
				p.Name,
				p.SchemaName,
				p.CreatedAt.Local().Format("2006-01-02"),
				p.Role,
			}
			if withStats && p.Stats != nil {
				row = append(row,
					fmt.Sprintf("%d", p.Stats.ContextCount),
					fmt.Sprintf("%d", p.Stats.SessionCount),
					fmt.Sprintf("%d", p.Stats.FileCount),
				)
			}
			rows = append(rows, row)
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nTotal: %d projects\n", resp.Total)
		}
		return nil
	},
}

var projectInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show project details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		withStats, _ := cmd.Flags().GetBool("stats")

		resp, err := apiClient.GetProject(args[0], withStats)
		if err != nil {
			return err
		}

		f := getFormatter()
		fields := []output.KeyValue{
			{Key: "Name", Value: resp.Name},
			{Key: "Schema", Value: resp.SchemaName},
			{Key: "Created", Value: resp.CreatedAt.Local().Format("2006-01-02 15:04:05")},
			{Key: "Role", Value: resp.Role},
			{Key: "System", Value: fmt.Sprintf("%v", resp.IsSystem)},
		}
		if resp.Description != nil {
			fields = append(fields, output.KeyValue{Key: "Description", Value: *resp.Description})
		}
		if withStats && resp.Stats != nil {
			fields = append(fields,
				output.KeyValue{Key: "Contexts", Value: fmt.Sprintf("%d", resp.Stats.ContextCount)},
				output.KeyValue{Key: "Sessions", Value: fmt.Sprintf("%d", resp.Stats.SessionCount)},
				output.KeyValue{Key: "Files", Value: fmt.Sprintf("%d", resp.Stats.FileCount)},
				output.KeyValue{Key: "DB Storage", Value: formatBytes(resp.Stats.StorageBytesDB)},
				output.KeyValue{Key: "S3 Storage", Value: formatBytes(resp.Stats.StorageBytesS3)},
			)
			if resp.Stats.LastActivity != nil {
				fields = append(fields, output.KeyValue{Key: "Last Activity", Value: resp.Stats.LastActivity.Local().Format("2006-01-02 15:04:05")})
			}
		}

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return fmt.Errorf("must pass --confirm to delete project %q", args[0])
		}

		if err := apiClient.DeleteProject(args[0]); err != nil {
			return err
		}

		fmt.Printf("Project %q deleted.\n", args[0])
		return nil
	},
}

var projectUseCmd = &cobra.Command{
	Use:   "use <name>",
	Short: "Set the default project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.SetValue("default_project", args[0]); err != nil {
			return fmt.Errorf("saving default project: %w", err)
		}
		fmt.Printf("Default project set to %q\n", args[0])
		return nil
	},
}

// ProjectBriefResponse is the MCP project_brief tool response.
type ProjectBriefResponse struct {
	Project      string `json:"project"`
	Summary      string `json:"summary"`
	ContextCount int    `json:"context_count"`
	TopTopics    []struct {
		Topic    string `json:"topic"`
		Priority string `json:"priority"`
	} `json:"top_topics,omitempty"`
	GeneratedAt string `json:"generated_at,omitempty"`
	Cached      bool   `json:"cached,omitempty"`
}

var projectBriefCmd = &cobra.Command{
	Use:   "brief [name]",
	Short: "Show LLM-synthesized project overview",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project := ""
		if len(args) > 0 {
			project = args[0]
		} else {
			var err error
			project, err = getProject()
			if err != nil {
				return err
			}
		}

		refresh, _ := cmd.Flags().GetBool("refresh")
		mcpArgs := map[string]any{"project": project}
		if refresh {
			mcpArgs["refresh"] = true
		}

		var resp ProjectBriefResponse
		if err := mcpClient.CallToolTyped("project_brief", mcpArgs, &resp); err != nil {
			var raw *api.NonJSONToolResult
			if errors.As(err, &raw) {
				fmt.Println(raw.Text) // server-rendered text (STOMPY-1921)
				return nil
			}
			return err
		}

		f := getFormatter()
		fields := []output.KeyValue{
			{Key: "Project", Value: resp.Project},
			{Key: "Contexts", Value: fmt.Sprintf("%d", resp.ContextCount)},
		}
		if resp.GeneratedAt != "" {
			fields = append(fields, output.KeyValue{Key: "Generated", Value: resp.GeneratedAt})
		}
		if resp.Cached {
			fields = append(fields, output.KeyValue{Key: "Source", Value: "cached"})
		}
		fields = append(fields, output.KeyValue{Key: "Summary", Value: resp.Summary})

		if len(resp.TopTopics) > 0 && isTableOutput() {
			fmt.Print(f.FormatSingle(fields))
			fmt.Println()
			headers := []string{"TOPIC", "PRIORITY"}
			var rows [][]string
			for _, t := range resp.TopTopics {
				rows = append(rows, []string{t.Topic, t.Priority})
			}
			fmt.Print(f.FormatTable(headers, rows))
		} else {
			fmt.Print(f.FormatSingle(fields))
		}
		return nil
	},
}

func init() {
	projectCreateCmd.Flags().String("description", "", "Project description")
	projectListCmd.Flags().Bool("stats", false, "Include project statistics")
	projectInfoCmd.Flags().Bool("stats", false, "Include project statistics")
	projectDeleteCmd.Flags().Bool("confirm", false, "Confirm deletion (required)")
	projectBriefCmd.Flags().Bool("refresh", false, "Force regeneration of the brief")

	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectInfoCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectUseCmd)
	projectCmd.AddCommand(projectBriefCmd)
	rootCmd.AddCommand(projectCmd)
}

func formatBytes(b int) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := unit, 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
