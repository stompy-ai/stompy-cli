package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/banton/stompy-cli/internal/output"
	"github.com/spf13/cobra"
)

// parseTopicRef parses a deeplink topic reference into project, topic, and version components.
// Supported formats:
//
//	"project/topic"        → project="project", topic="topic", version=""
//	"_global/topic"        → project="_global", topic="topic", version=""
//	"project/topic@v1.0"   → project="project", topic="topic", version="v1.0"
//	"plain_topic"          → project=projectFlag, topic="plain_topic", version=""
//	"plain_topic@v2"       → project=projectFlag, topic="plain_topic", version="v2"
//
// If a deeplink project is extracted AND projectFlag is non-empty, the deeplink project wins
// and a warning is printed.
func parseTopicRef(ref, projectFlag string) (project, topic, version string) {
	// Split off version suffix (@version)
	base := ref
	if idx := strings.LastIndex(ref, "@"); idx != -1 {
		version = ref[idx+1:]
		base = ref[:idx]
	}

	// Check for project/topic deeplink syntax
	if idx := strings.Index(base, "/"); idx != -1 {
		dlProject := base[:idx]
		dlTopic := base[idx+1:]
		if projectFlag != "" && projectFlag != dlProject {
			fmt.Fprintf(os.Stderr, "warning: deeplink project %q overrides --project %q\n", dlProject, projectFlag)
		}
		return dlProject, dlTopic, version
	}

	// Plain topic — use projectFlag as-is
	return projectFlag, base, version
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Manage contexts (persistent memory)",
}

var contextLockCmd = &cobra.Command{
	Use:   "lock <topic>",
	Short: "Lock (create) a context",
	Long: `Lock (create or update) a context. Accepts deeplink syntax:

  stompy context lock project/topic --content "..."
  stompy context lock _global/topic --content "..."
  stompy context lock plain-topic --content "..."`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFlag, err := getProject()
		if err != nil {
			return err
		}

		project, topic, _ := parseTopicRef(args[0], projectFlag)

		content, err := resolveContent(cmd)
		if err != nil {
			return err
		}

		tags, _ := cmd.Flags().GetString("tags")
		priority, _ := cmd.Flags().GetString("priority")
		force, _ := cmd.Flags().GetBool("force")

		req := api.ContextCreateRequest{
			Topic:      topic,
			Content:    content,
			Tags:       tags,
			Priority:   priority,
			ForceStore: force,
		}

		resp, err := apiClient.LockContext(project, req)
		if err != nil {
			return err
		}

		fmt.Printf("%s Context locked: %s (version %s)\n", output.Success("✓"), output.Teal(resp.Topic), resp.Version)
		return nil
	},
}

var contextRecallCmd = &cobra.Command{
	Use:   "recall <topic>",
	Short: "Recall (read) a context",
	Long: `Recall (read) a context. Accepts deeplink syntax:

  stompy context recall project/topic
  stompy context recall _global/topic
  stompy context recall project/topic@v1.0
  stompy context recall plain-topic@v2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFlag, err := getProject()
		if err != nil {
			return err
		}

		versionFlag, _ := cmd.Flags().GetString("version")
		project, topic, versionRef := parseTopicRef(args[0], projectFlag)

		// --version flag takes precedence over @version suffix in deeplink
		version := versionFlag
		if version == "" {
			version = versionRef
		}

		resp, err := apiClient.GetContext(project, topic, version)
		if err != nil {
			return err
		}

		f := getFormatter()
		fields := []output.KeyValue{
			{Key: "Topic", Value: resp.Topic},
			{Key: "Version", Value: resp.Version},
			{Key: "Priority", Value: resp.Priority},
		}
		if len(resp.Tags) > 0 {
			fields = append(fields, output.KeyValue{Key: "Tags", Value: strings.Join(resp.Tags, ", ")})
		}
		fields = append(fields, output.KeyValue{Key: "Content", Value: resp.Content})

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

var contextUnlockCmd = &cobra.Command{
	Use:   "unlock <topic>",
	Short: "Unlock (delete) a context",
	Long: `Unlock (delete) a context. Accepts deeplink syntax:

  stompy context unlock project/topic
  stompy context unlock _global/topic`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFlag, err := getProject()
		if err != nil {
			return err
		}

		project, topic, _ := parseTopicRef(args[0], projectFlag)

		version, _ := cmd.Flags().GetString("version")
		force, _ := cmd.Flags().GetBool("force")
		noArchive, _ := cmd.Flags().GetBool("no-archive")

		resp, err := apiClient.UnlockContext(project, topic, version, force, noArchive)
		if err != nil {
			return err
		}

		archivedStr := ""
		if resp.Archived {
			archivedStr = " (archived)"
		}
		fmt.Printf("%s Context unlocked: %s%s\n", output.Success("✓"), output.Teal(resp.Topic), archivedStr)
		return nil
	},
}

var contextListCmd = &cobra.Command{
	Use:   "list",
	Short: "List contexts",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		priority, _ := cmd.Flags().GetString("priority")
		tags, _ := cmd.Flags().GetString("tags")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")
		fresh, _ := cmd.Flags().GetBool("fresh")

		if fresh {
			apiClient.NoCache = true
		}

		resp, err := apiClient.ListContexts(project, priority, tags, limit, offset)
		if err != nil {
			return err
		}

		f := getFormatter()
		headers := []string{"ID", "TOPIC", "VERSION", "PRIORITY", "TAGS", "ACCESS COUNT"}
		var rows [][]string
		for _, c := range resp.Contexts {
			rows = append(rows, []string{
				fmt.Sprintf("%d", c.ID),
				c.Topic,
				c.Version,
				c.Priority,
				strings.Join(c.Tags, ", "),
				fmt.Sprintf("%d", c.AccessCount),
			})
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nTotal: %d contexts\n", resp.Total)
		}
		return nil
	},
}

var contextSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search contexts",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")

		resp, err := apiClient.SearchContexts(project, args[0], limit)
		if err != nil {
			return err
		}

		f := getFormatter()
		headers := []string{"ID", "TOPIC", "PRIORITY", "PREVIEW"}
		var rows [][]string
		for _, c := range resp.Contexts {
			preview := ""
			if c.Preview != nil {
				preview = *c.Preview
				if len(preview) > 60 {
					preview = preview[:57] + "..."
				}
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", c.ID),
				c.Topic,
				c.Priority,
				preview,
			})
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nFound: %d contexts\n", resp.Total)
		}
		return nil
	},
}

var contextUpdateCmd = &cobra.Command{
	Use:   "update <topic>",
	Short: "Update a context",
	Long: `Update a context. Accepts deeplink syntax:

  stompy context update project/topic --content "..."
  stompy context update _global/topic --content "..."`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFlag, err := getProject()
		if err != nil {
			return err
		}

		project, topic, _ := parseTopicRef(args[0], projectFlag)

		content, err := resolveContent(cmd)
		if err != nil {
			return err
		}

		priority, _ := cmd.Flags().GetString("priority")
		tags, _ := cmd.Flags().GetString("tags")

		req := api.ContextUpdateRequest{
			Content:  content,
			Priority: priority,
			Tags:     tags,
		}

		resp, err := apiClient.UpdateContext(project, topic, req)
		if err != nil {
			return err
		}

		fmt.Printf("%s Context updated: %s (version %s)\n", output.Success("✓"), output.Teal(resp.Topic), resp.Version)
		return nil
	},
}

var contextMoveCmd = &cobra.Command{
	Use:   "move <topic>",
	Short: "Move a context to another project",
	Long: `Move a context to another project. Accepts deeplink syntax:

  stompy context move project/topic --to other-project
  stompy context move _global/topic --to my-project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectFlag, err := getProject()
		if err != nil {
			return err
		}

		project, topic, _ := parseTopicRef(args[0], projectFlag)

		target, _ := cmd.Flags().GetString("to")
		if target == "" {
			return fmt.Errorf("--to flag is required")
		}

		resp, err := apiClient.MoveContext(project, topic, target)
		if err != nil {
			return err
		}

		fmt.Printf("%s Context %s moved to project %s\n", output.Success("✓"), output.Teal(resp.Topic), output.Teal(resp.TargetProject))
		return nil
	},
}

// ContextExploreResponse is the MCP context_explore tool response.
type ContextExploreResponse struct {
	Project  string `json:"project"`
	Contexts []struct {
		Topic       string `json:"topic"`
		Priority    string `json:"priority"`
		Version     string `json:"version"`
		Tags        string `json:"tags,omitempty"`
		Preview     string `json:"preview,omitempty"`
		AccessCount int    `json:"access_count"`
	} `json:"contexts"`
	Total int `json:"total"`
}

var contextExploreCmd = &cobra.Command{
	Use:   "explore",
	Short: "Browse contexts by priority with tree view",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		grep, _ := cmd.Flags().GetString("grep")
		verbose, _ := cmd.Flags().GetBool("verbose")

		mcpArgs := map[string]any{"project": project}
		if grep != "" {
			mcpArgs["grep"] = grep
		}
		if verbose {
			mcpArgs["verbose"] = true
		}

		var resp ContextExploreResponse
		if err := mcpClient.CallToolTyped("context_explore", mcpArgs, &resp); err != nil {
			var raw *api.NonJSONToolResult
			if errors.As(err, &raw) {
				fmt.Println(raw.Text) // server-rendered text (STOMPY-1921)
				return nil
			}
			return err
		}

		f := getFormatter()
		headers := []string{"TOPIC", "PRIORITY", "VERSION", "TAGS", "ACCESSES"}
		if verbose {
			headers = append(headers, "PREVIEW")
		}

		var rows [][]string
		for _, c := range resp.Contexts {
			row := []string{
				c.Topic,
				c.Priority,
				c.Version,
				c.Tags,
				fmt.Sprintf("%d", c.AccessCount),
			}
			if verbose {
				preview := c.Preview
				if len(preview) > 50 {
					preview = preview[:47] + "..."
				}
				row = append(row, preview)
			}
			rows = append(rows, row)
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nTotal: %d contexts in %s\n", resp.Total, resp.Project)
		}
		return nil
	},
}

// ContextDashboardResponse is the MCP context_dashboard tool response.
type ContextDashboardResponse struct {
	Project       string         `json:"project"`
	TotalContexts int            `json:"total_contexts"`
	ByPriority    map[string]int `json:"by_priority"`
	RecentTopics  []string       `json:"recent_topics,omitempty"`
	StaleCount    int            `json:"stale_count"`
}

var contextDashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Show context statistics and health",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		detail, _ := cmd.Flags().GetString("detail")
		mcpArgs := map[string]any{"project": project}
		if detail != "" {
			mcpArgs["detail"] = detail
		}

		var resp ContextDashboardResponse
		if err := mcpClient.CallToolTyped("context_dashboard", mcpArgs, &resp); err != nil {
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
			{Key: "Total Contexts", Value: fmt.Sprintf("%d", resp.TotalContexts)},
			{Key: "Stale Contexts", Value: fmt.Sprintf("%d", resp.StaleCount)},
		}

		// Priority breakdown
		for _, p := range []string{"always_check", "important", "reference", "nice_to_have"} {
			if count, ok := resp.ByPriority[p]; ok {
				fields = append(fields, output.KeyValue{Key: fmt.Sprintf("  %s", p), Value: fmt.Sprintf("%d", count)})
			}
		}

		if len(resp.RecentTopics) > 0 {
			fields = append(fields, output.KeyValue{Key: "Recent Topics", Value: strings.Join(resp.RecentTopics, ", ")})
		}

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

// RecallBatchItem is one topic's entry in a recall_batch result.
type RecallBatchItem struct {
	Topic   string `json:"topic"`
	Content string `json:"content,omitempty"`
	Preview string `json:"preview,omitempty"`
	Version string `json:"version,omitempty"`
	Found   bool   `json:"found"`
	Error   string `json:"error,omitempty"`
}

// RecallBatchResponse is the MCP recall_batch tool response. Since 6.6.x the
// server returns `results` as an OBJECT keyed by topic plus `found` /
// `not_found` lists (STOMPY-1921); older builds returned a list. Both parse.
type RecallBatchResponse struct {
	Results []RecallBatchItem
}

func (r *RecallBatchResponse) UnmarshalJSON(b []byte) error {
	var asList struct {
		Results []RecallBatchItem `json:"results"`
	}
	if err := json.Unmarshal(b, &asList); err == nil && asList.Results != nil {
		r.Results = asList.Results
		return nil
	}
	var asMap struct {
		Results  map[string]RecallBatchItem `json:"results"`
		Found    []string                   `json:"found"`
		NotFound []string                   `json:"not_found"`
	}
	if err := json.Unmarshal(b, &asMap); err != nil {
		return err
	}
	order := asMap.Found
	if len(order) == 0 {
		for k := range asMap.Results {
			order = append(order, k)
		}
		sort.Strings(order)
	}
	for _, topic := range order {
		item, ok := asMap.Results[topic]
		if !ok {
			continue
		}
		if item.Topic == "" {
			item.Topic = topic
		}
		item.Found = true
		r.Results = append(r.Results, item)
	}
	for _, topic := range asMap.NotFound {
		r.Results = append(r.Results, RecallBatchItem{Topic: topic, Found: false})
	}
	return nil
}

var contextBatchCmd = &cobra.Command{
	Use:   "batch <topic1> <topic2> ...",
	Short: "Fetch multiple contexts in one call",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		preview, _ := cmd.Flags().GetBool("preview")
		mcpArgs := map[string]any{
			"project": project,
			"topics":  args,
		}
		if preview {
			mcpArgs["preview_only"] = true
		}

		var resp RecallBatchResponse
		if err := mcpClient.CallToolTyped("recall_batch", mcpArgs, &resp); err != nil {
			var raw *api.NonJSONToolResult
			if errors.As(err, &raw) {
				fmt.Println(raw.Text) // server-rendered text (STOMPY-1921)
				return nil
			}
			return err
		}

		f := getFormatter()

		if preview {
			headers := []string{"TOPIC", "FOUND", "VERSION", "PREVIEW"}
			var rows [][]string
			for _, r := range resp.Results {
				found := "no"
				if r.Found {
					found = "yes"
				}
				p := r.Preview
				if len(p) > 60 {
					p = p[:57] + "..."
				}
				rows = append(rows, []string{r.Topic, found, r.Version, p})
			}
			fmt.Print(f.FormatTable(headers, rows))
		} else {
			for i, r := range resp.Results {
				if i > 0 && isTableOutput() {
					fmt.Println(strings.Repeat("─", 40))
				}
				if !r.Found {
					fmt.Printf("%s %s: not found", output.Warn("!"), r.Topic)
					if r.Error != "" {
						fmt.Printf(" (%s)", r.Error)
					}
					fmt.Println()
					continue
				}
				fields := []output.KeyValue{
					{Key: "Topic", Value: r.Topic},
					{Key: "Version", Value: r.Version},
					{Key: "Content", Value: r.Content},
				}
				fmt.Print(f.FormatSingle(fields))
			}
		}
		return nil
	},
}

func init() {
	contextLockCmd.Flags().String("content", "", "Context content (use @file to read from file)")
	contextLockCmd.Flags().String("tags", "", "Comma-separated tags")
	contextLockCmd.Flags().String("priority", "", "Priority: always_check, important, reference, nice_to_have")
	contextLockCmd.Flags().Bool("force", false, "Force store even if similar content exists")

	contextRecallCmd.Flags().String("version", "", "Specific version to recall")

	contextUnlockCmd.Flags().String("version", "", "Version to unlock: all, latest, or specific version")
	contextUnlockCmd.Flags().Bool("force", false, "Force unlock without confirmation")
	contextUnlockCmd.Flags().Bool("no-archive", false, "Delete without archiving")

	contextListCmd.Flags().String("priority", "", "Filter by priority")
	contextListCmd.Flags().String("tags", "", "Filter by tags")
	contextListCmd.Flags().Int("limit", 0, "Limit results")
	contextListCmd.Flags().Int("offset", 0, "Offset for pagination")
	contextListCmd.Flags().Bool("fresh", false, "Bypass server cache for fresh results")

	contextSearchCmd.Flags().Int("limit", 0, "Limit results")

	contextUpdateCmd.Flags().String("content", "", "New content (use @file to read from file)")
	contextUpdateCmd.Flags().String("priority", "", "New priority")
	contextUpdateCmd.Flags().String("tags", "", "New tags")

	contextMoveCmd.Flags().String("to", "", "Target project name (required)")

	contextExploreCmd.Flags().String("grep", "", "Filter by topic glob pattern")
	contextExploreCmd.Flags().Bool("verbose", false, "Include content previews")

	contextDashboardCmd.Flags().String("detail", "", "Detail level: summary, verbose")

	contextBatchCmd.Flags().Bool("preview", false, "Preview-only mode (no full content)")

	contextCmd.AddCommand(contextLockCmd)
	contextCmd.AddCommand(contextRecallCmd)
	contextCmd.AddCommand(contextUnlockCmd)
	contextCmd.AddCommand(contextListCmd)
	contextCmd.AddCommand(contextSearchCmd)
	contextCmd.AddCommand(contextUpdateCmd)
	contextCmd.AddCommand(contextMoveCmd)
	contextCmd.AddCommand(contextExploreCmd)
	contextCmd.AddCommand(contextDashboardCmd)
	contextCmd.AddCommand(contextBatchCmd)
	rootCmd.AddCommand(contextCmd)
}

// resolveContent reads content from --content flag, @file reference, or stdin.
func resolveContent(cmd *cobra.Command) (string, error) {
	contentFlag, _ := cmd.Flags().GetString("content")

	// Check if stdin has data (pipe)
	if contentFlag == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return "", fmt.Errorf("reading stdin: %w", err)
			}
			return string(data), nil
		}
		return "", fmt.Errorf("--content flag is required (or pipe content via stdin)")
	}

	// @file reference
	if strings.HasPrefix(contentFlag, "@") {
		filePath := strings.TrimPrefix(contentFlag, "@")
		data, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("reading file %q: %w", filePath, err)
		}
		return string(data), nil
	}

	return contentFlag, nil
}
