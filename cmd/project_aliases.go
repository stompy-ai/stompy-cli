package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

var projectAliasesCmd = &cobra.Command{
	Use:   "aliases <name>",
	Short: "List the live old names of a project you own",
	Args:  exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("project name must not be empty")
		}
		result, err := apiClient.GetProjectAliases(args[0])
		if err != nil {
			return err
		}
		if result.Name == "" || result.SchemaName == "" || result.Aliases == nil {
			return fmt.Errorf("live alias list is unverifiable")
		}
		sort.Strings(result.Aliases)
		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(result))
			return nil
		}
		aliases := strings.Join(result.Aliases, ", ")
		if len(result.Aliases) == 0 {
			aliases = "none"
		}
		fmt.Print(getFormatter().FormatSingle([]output.KeyValue{{Key: "Name", Value: result.Name}, {Key: "Live aliases", Value: aliases}}))
		return nil
	},
}

func init() { projectCmd.AddCommand(projectAliasesCmd) }
