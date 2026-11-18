package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var projectReleaseNameCmd = &cobra.Command{
	Use:   "release-name <alias>",
	Short: "Stop forwarding an old project name (owner only)",
	Long:  "Release an alias of the project selected by --project or the existing project binding. Old links stop working. Requires --confirm; never edits .stompy.yaml.",
	Args:  exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return fmt.Errorf("must pass --confirm: old links stop working when an alias is released")
		}
		if strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("alias must not be empty")
		}
		project, err := getProject()
		if err != nil {
			return err
		}
		before, err := apiClient.GetProject(project, false)
		if err != nil {
			return err
		}
		if err = usableNameSnapshot(before); err != nil {
			return err
		}
		result, err := apiClient.ReleaseProjectName(project, args[0])
		if err != nil {
			return fmt.Errorf("release not confirmed; inspect live aliases before retrying: %w", err)
		}
		if !result.Success || result.Name == "" || result.SchemaName != before.SchemaName || result.ReleasedAlias != args[0] {
			return fmt.Errorf("alias release outcome is unverifiable; inspect project info and aliases before another mutation")
		}
		warnPendingNameWrite(result)
		aliases, err := verifyProjectName(project, before, result)
		if err != nil {
			return fmt.Errorf("alias release committed but readback is unverified; inspect project info and aliases, do not repeat the release: %w", err)
		}
		for _, alias := range aliases {
			if alias == args[0] {
				return fmt.Errorf("alias release readback is unverified: the alias is still live; do not repeat the release")
			}
		}
		return printVerifiedName(result, aliases)
	},
}

func init() {
	projectReleaseNameCmd.Flags().Bool("confirm", false, "Confirm that old links stop working")
	projectCmd.AddCommand(projectReleaseNameCmd)
}
