package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

type verifiedProjectName struct {
	*api.ProjectNameResult
	Aliases  []string `json:"aliases"`
	Verified bool     `json:"verified"`
}

var projectRenameCmd = &cobra.Command{
	Use:   "rename <name> <new>",
	Short: "Rename a project you own (the old name keeps forwarding)",
	Args:  exactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("project and new name must not be empty")
		}
		before, err := apiClient.GetProject(args[0], false)
		if err != nil {
			return err
		}
		if err = usableNameSnapshot(before); err != nil {
			return err
		}
		result, err := apiClient.RenameProject(args[0], args[1])
		if err != nil {
			return err
		}
		if !result.Success || result.Name == "" || result.SchemaName != before.SchemaName {
			return fmt.Errorf("rename outcome is unverifiable; inspect project info before another mutation")
		}
		if result.MetadataSync == "pending" {
			fmt.Fprintln(os.Stderr, "Rename committed; metadata synchronization is pending. Do not repeat the rename.")
		}
		aliases, err := verifyProjectName(args[0], before, result)
		if err != nil {
			return fmt.Errorf("rename committed but readback is unverified; use project info to verify, do not repeat the rename: %w", err)
		}
		return printVerifiedName(result, aliases)
	},
}

func usableNameSnapshot(p *api.ProjectResponse) error {
	if p.Name == "" || p.SchemaName == "" || p.CreatedAt.IsZero() || p.EnrichmentWarning != "" {
		return fmt.Errorf("project identity/display is unverifiable; no name mutation was sent")
	}
	return nil
}

func scopedName(address, newName string) string {
	owner, _ := nameParts(address)
	if owner != "" {
		return "@" + owner + "/" + newName
	}
	return newName
}

func verifyProjectName(address string, before *api.ProjectResponse, result *api.ProjectNameResult) ([]string, error) {
	currentAddress := scopedName(address, result.Name)
	current, err := apiClient.GetProject(currentAddress, false)
	if err != nil {
		return nil, err
	}
	if current.Name != result.Name || current.SchemaName != result.SchemaName || current.NowCalled != "" || current.EnrichmentWarning != "" {
		return nil, fmt.Errorf("project identity or current name changed")
	}
	if !current.CreatedAt.Equal(before.CreatedAt) {
		return nil, fmt.Errorf("project was deleted and recreated")
	}
	aliases, err := apiClient.GetProjectAliases(currentAddress)
	if err != nil {
		return nil, err
	}
	if aliases.Name != result.Name || aliases.SchemaName != result.SchemaName || aliases.Aliases == nil {
		return nil, fmt.Errorf("live alias readback is unverifiable")
	}
	return aliases.Aliases, nil
}

func printVerifiedName(result *api.ProjectNameResult, aliases []string) error {
	if !isTableOutput() {
		fmt.Print(getFormatter().FormatRaw(verifiedProjectName{result, aliases, true}))
		return nil
	}
	fields := []output.KeyValue{{Key: "Name", Value: result.Name}, {Key: "Schema", Value: result.SchemaName}, {Key: "Aliases", Value: strings.Join(aliases, ", ")}, {Key: "Verified", Value: "true"}}
	if result.MetadataSync != "" {
		fields = append(fields, output.KeyValue{Key: "Metadata sync", Value: result.MetadataSync})
	}
	fmt.Print(getFormatter().FormatSingle(fields))
	return nil
}

func init() { projectCmd.AddCommand(projectRenameCmd) }
