package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// STOMPY-3125: a context's links were readable only on MCP (recall_context's
// "Links:" block). This reads the REST door of the same edges.
var contextLinksCmd = &cobra.Command{
	Use:   "links <topic>",
	Short: "Show a context's typed links (outbound and cited-by)",
	Long: `Show a context's typed links. Accepts deeplink syntax:

  stompy context links project/topic`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, topic, _, err := resolveTopicRef(args[0])
		if err != nil {
			return err
		}
		resp, err := apiClient.GetContextLinks(project, topic)
		if err != nil {
			return err
		}
		f := getFormatter()
		if !isTableOutput() {
			fmt.Print(f.FormatRaw(resp))
			return nil
		}
		str := func(m map[string]any, k string) string {
			if v, ok := m[k]; ok && v != nil {
				return fmt.Sprintf("%v", v)
			}
			return ""
		}
		var rows [][]string
		for _, direction := range []string{"links", "cited_by"} {
			edges, _ := resp[direction].([]any)
			for _, e := range edges {
				m, _ := e.(map[string]any)
				rows = append(rows, []string{direction, str(m, "topic"), str(m, "link_type"), str(m, "provenance"), str(m, "weight")})
			}
		}
		fmt.Print(f.FormatTable([]string{"DIRECTION", "TOPIC", "LINK TYPE", "PROVENANCE", "WEIGHT"}, rows))
		if truncated, _ := resp["truncated"].(bool); truncated {
			fmt.Println("(more links exist than shown)")
		}
		return nil
	},
}

func init() {
	contextCmd.AddCommand(contextLinksCmd)
}
