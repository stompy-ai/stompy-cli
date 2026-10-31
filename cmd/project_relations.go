package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

// STOMPY-2712 (door parity: same tools on every feasible touchpoint):
// `project set-metadata`, `project link`, `project unlink`, `project links`
// over the existing REST routes. The server decides who may write where; a
// refusal (including STOMPY-2746's foreign-address refusal) reaches the user
// verbatim and is never retried or rewritten.

// sourceAndRest resolves the source project: with `want` positionals left
// after the source, a surplus first arg is the source; otherwise the source
// comes from -p / STOMPY_PROJECT / .stompy.yaml / default (config.ResolveProject).
func sourceAndRest(args []string, want int) (string, []string, error) {
	if len(args) == want+1 {
		return args[0], args[1:], nil
	}
	p, err := getProject()
	if err != nil {
		return "", nil, err
	}
	return p, args, nil
}

func strFlag(cmd *cobra.Command, name string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	v, _ := cmd.Flags().GetString(name)
	return &v
}

var projectSetMetadataCmd = &cobra.Command{
	Use:   "set-metadata [project]",
	Short: "Set elicited project metadata (origin, category, lifecycle state, purpose, intent)",
	Long: `Set the project's elicited KG metadata. Omitted fields are unchanged; names
passed to --clear are unset. The project defaults to the resolved project
(-p, STOMPY_PROJECT, .stompy.yaml, default project).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _, err := sourceAndRest(args, 0)
		if err != nil {
			return err
		}
		clear, _ := cmd.Flags().GetStringSlice("clear")
		resp, err := apiClient.SetProjectMetadata(project, api.ProjectMetadataUpdate{
			Origin:         strFlag(cmd, "origin"),
			Category:       strFlag(cmd, "category"),
			LifecycleState: strFlag(cmd, "lifecycle-state"),
			Purpose:        strFlag(cmd, "purpose"),
			Intent:         strFlag(cmd, "intent"),
			Clear:          clear,
		})
		if err != nil {
			return err
		}
		f := getFormatter()
		if !isTableOutput() {
			fmt.Print(f.FormatRaw(resp))
			return nil
		}
		kg, _ := resp["kg"].(map[string]any)
		fields := []output.KeyValue{{Key: "Project", Value: project}}
		for _, k := range []string{"origin", "category", "lifecycle_state", "purpose", "intent"} {
			v := "-"
			if s, ok := kg[k].(string); ok && s != "" {
				v = s
			}
			fields = append(fields, output.KeyValue{Key: k, Value: v})
		}
		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

var projectLinkCmd = &cobra.Command{
	Use:   "link [source] <target>",
	Short: "Relate one of your projects to another (directed, idempotent)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		source, rest, err := sourceAndRest(args, 1)
		if err != nil {
			return err
		}
		rel, _ := cmd.Flags().GetString("relation-type")
		if strings.TrimSpace(rel) == "" {
			return fmt.Errorf("--relation-type is required (e.g. 'grew out of', 'feeds', 'supersedes')")
		}
		resp, err := apiClient.LinkProject(source, api.ProjectEdgeCreate{
			Target:       rest[0],
			RelationType: rel,
			Note:         strFlag(cmd, "note"),
			Grounding:    strFlag(cmd, "grounding"),
		})
		if err != nil {
			return err
		}
		f := getFormatter()
		if !isTableOutput() {
			fmt.Print(f.FormatRaw(resp))
			return nil
		}
		fields := []output.KeyValue{{Key: "From", Value: source}, {Key: "To", Value: rest[0]}, {Key: "Relation", Value: rel}}
		if edge, ok := resp["edge"].(map[string]any); ok {
			if n, ok := edge["note"].(string); ok && n != "" {
				fields = append(fields, output.KeyValue{Key: "Note", Value: n})
			}
		}
		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

var projectUnlinkCmd = &cobra.Command{
	Use:   "unlink [source] <target>",
	Short: "Remove relation(s) from a project to a target (0 removed is not an error)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		source, rest, err := sourceAndRest(args, 1)
		if err != nil {
			return err
		}
		rel, _ := cmd.Flags().GetString("relation-type")
		resp, err := apiClient.UnlinkProject(source, rest[0], rel)
		if err != nil {
			return err
		}
		f := getFormatter()
		if !isTableOutput() {
			fmt.Print(f.FormatRaw(resp))
			return nil
		}
		fmt.Printf("Removed %v relation(s) from %q to %q.\n", resp["removed"], source, rest[0])
		return nil
	},
}

var projectLinksCmd = &cobra.Command{
	Use:   "links [project]",
	Short: "List the project's relations (both directions)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, _, err := sourceAndRest(args, 0)
		if err != nil {
			return err
		}
		resp, err := apiClient.ListProjectEdges(project)
		if err != nil {
			return err
		}
		f := getFormatter()
		if !isTableOutput() {
			fmt.Print(f.FormatRaw(resp))
			return nil
		}
		str := func(m map[string]any, k string) string {
			if s, ok := m[k].(string); ok {
				return s
			}
			return ""
		}
		var rows [][]string
		edges, _ := resp["edges"].([]any)
		for _, e := range edges {
			m, _ := e.(map[string]any)
			rows = append(rows, []string{str(m, "source"), str(m, "relation_type"), str(m, "target"), str(m, "note")})
		}
		fmt.Print(f.FormatTable([]string{"FROM", "RELATION", "TO", "NOTE"}, rows))
		return nil
	},
}

func init() {
	projectSetMetadataCmd.Flags().String("origin", "", "human | agent | artifact | unknown")
	projectSetMetadataCmd.Flags().String("category", "", "Free-text category")
	projectSetMetadataCmd.Flags().String("lifecycle-state", "", "Free-text lifecycle/completeness state")
	projectSetMetadataCmd.Flags().String("purpose", "", "What the project is for")
	projectSetMetadataCmd.Flags().String("intent", "", "Intent behind the project")
	projectSetMetadataCmd.Flags().StringSlice("clear", nil, "Field names to unset (comma-separated or repeated)")

	projectLinkCmd.Flags().String("relation-type", "", "Free-text relation, e.g. 'feeds', 'grew out of' (required)")
	projectLinkCmd.Flags().String("note", "", "Why the relation holds")
	projectLinkCmd.Flags().String("grounding", "", "'corpus' or 'session' (only when answering project_create's question)")
	projectUnlinkCmd.Flags().String("relation-type", "", "Remove only this relation (default: every relation to the target)")

	projectCmd.AddCommand(projectSetMetadataCmd, projectLinkCmd, projectUnlinkCmd, projectLinksCmd)
}
