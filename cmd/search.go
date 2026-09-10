package cmd

import (
	"fmt"

	"github.com/banton/stompy-cli/internal/output"
	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across contexts, tickets, and files",
	Long:  `Perform hybrid semantic + keyword search across a project's contexts, tickets, and files.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")

		resp, err := apiClient.Search(project, args[0], limit)
		if err != nil {
			return err
		}

		f := getFormatter()
		headers := []string{"ID", "LABEL", "PRIORITY", "PREVIEW", "SCORE", "SOURCE", "SCOPE"}
		var rows [][]string
		for _, r := range resp.Results {
			preview := r.Preview
			if len(preview) > 60 {
				preview = preview[:57] + "..."
			}
			priority := r.DecodedMetadata().Priority
			if isTableOutput() && priority != "" {
				priority = output.ColorPriority(priority)
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", r.ID),
				r.Label,
				priority,
				preview,
				fmt.Sprintf("%.4f", r.RankScore()),
				r.Source,
				r.Scope,
			})
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nFound: %d results for %q\n", resp.TotalFound, args[0])
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().Int("limit", 10, "Maximum number of results")
	rootCmd.AddCommand(searchCmd)
}
