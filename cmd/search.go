package cmd

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/banton/stompy-cli/internal/output"
	"github.com/spf13/cobra"
)

// searchRoute maps each flag to the GET /projects/{name}/search query
// parameter it sets. Every entry is a parameter the route declares
// (src/api/routes/search.py); a parameter is sent only when its flag was
// passed, so the route's own defaults stay the server's business.
var searchRoute = []struct {
	flag, param, kind, usage string
}{
	{"priority", "priority", "string", "Filter by priority (always_check, important, reference)"},
	{"tags", "tags", "string", "Comma-separated tags to filter by"},
	{"scope", "scope", "string", "project (default), project+global, or global"},
	{"threshold", "threshold", "float", "Minimum vector similarity, 0-1"},
	{"hybrid", "use_hybrid", "bool", "Use hybrid vector + keyword search (--hybrid=false for vector only)"},
	{"chunks", "search_chunks", "bool", "Search document chunks instead of contexts"},
	{"token-budget", "token_budget", "int", "Pack results to at most this many tokens (1000-200000)"},
	{"temporal", "use_temporal", "bool", "Rank recent contexts higher"},
	{"decay-rate", "temporal_decay_rate", "float", "Temporal decay rate, 0.001-1"},
	{"expand-entities", "expand_entities", "bool", "Expand results via shared entities"},
	{"expand-links", "expand_links", "bool", "Include 1-hop linked contexts"},
	{"explain", "explain", "bool", "Include a per-result signal breakdown in -o json"},
}

// addSearchFlags registers the search route's filters on cmd. `search` and
// `context search` are one capability (context.search) and share them.
func addSearchFlags(cmd *cobra.Command) {
	cmd.Flags().Int("limit", 10, "Maximum number of results (1-100)")
	for _, f := range searchRoute {
		switch f.kind {
		case "string":
			cmd.Flags().String(f.flag, "", f.usage)
		case "float":
			cmd.Flags().Float64(f.flag, 0, f.usage)
		case "int":
			cmd.Flags().Int(f.flag, 0, f.usage)
		case "bool":
			cmd.Flags().Bool(f.flag, f.param == "use_hybrid", f.usage)
		}
	}
}

// runSearch is the one implementation behind `stompy search` and
// `stompy context search` (STOMPY-2728): GET /projects/{name}/search?query=.
func runSearch(cmd *cobra.Command, query string) error {
	project, err := getProject()
	if err != nil {
		return err
	}

	params := url.Values{}
	if limit, _ := cmd.Flags().GetInt("limit"); limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	for _, f := range searchRoute {
		if !cmd.Flags().Changed(f.flag) {
			continue
		}
		switch f.kind {
		case "string":
			v, _ := cmd.Flags().GetString(f.flag)
			params.Set(f.param, v)
		case "float":
			v, _ := cmd.Flags().GetFloat64(f.flag)
			params.Set(f.param, strconv.FormatFloat(v, 'f', -1, 64))
		case "int":
			v, _ := cmd.Flags().GetInt(f.flag)
			params.Set(f.param, strconv.Itoa(v))
		case "bool":
			v, _ := cmd.Flags().GetBool(f.flag)
			params.Set(f.param, strconv.FormatBool(v))
		}
	}

	resp, err := apiClient.SearchWith(project, query, params)
	if err != nil {
		return err
	}

	if !isTableOutput() {
		fmt.Print(getFormatter().FormatRaw(resp.Raw))
		return nil
	}

	mode := resp.SearchMode
	if mode == "" {
		mode = "unknown"
	}
	if len(resp.Results) == 0 {
		fmt.Printf("no matches for %q (search_mode: %s)\n", query, mode)
		return nil
	}

	// SCOPE only earns a column when a hit came from outside the project
	// (scope=project+global); otherwise it is the same word on every row.
	showScope := false
	for _, r := range resp.Results {
		if r.Scope != "" && r.Scope != "project" {
			showScope = true
		}
	}
	headers := []string{"LABEL", "VERSION", "PRIORITY", "LIFECYCLE", "SCORE", "SOURCE"}
	if showScope {
		headers = append(headers, "SCOPE")
	}
	headers = append(headers, "PREVIEW")
	var rows [][]string
	for _, r := range resp.Results {
		preview := r.Preview
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		priority := r.DecodedMetadata().Priority
		if priority != "" {
			priority = output.ColorPriority(priority)
		}
		row := []string{
			r.Label,
			dash(r.Version),
			dash(priority),
			dash(r.Lifecycle),
			fmt.Sprintf("%.4f", r.RankScore()),
			dash(r.Source),
		}
		if showScope {
			row = append(row, dash(r.Scope))
		}
		rows = append(rows, append(row, preview))
	}
	fmt.Print(getFormatter().FormatTable(headers, rows))
	fmt.Printf("\nFound: %d results for %q (search_mode: %s)\n", resp.TotalFound, query, mode)
	return nil
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search contexts by meaning and keyword",
	Long: `Hybrid semantic + keyword search over a project's contexts
(GET /projects/{name}/search — the same route and ranking as MCP context_search).
Same command as 'stompy context search'.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error { return runSearch(cmd, args[0]) },
}

func init() {
	addSearchFlags(searchCmd)
	rootCmd.AddCommand(searchCmd)
}
