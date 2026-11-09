package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

// STOMPY-2705: project API key management. Owner-only and signed-in only —
// the server refuses an API-key principal (keys cannot manage keys), and that
// refusal reaches the user verbatim.

var projectKeyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage a project's API key (mint, info, revoke, request)",
}

var projectKeyMintCmd = &cobra.Command{
	Use:   "mint <project>",
	Short: "Mint the project's API key (ROTATES an existing key)",
	Long: `Mint the project's API key. If one is active, minting ROTATES it: the old
key stops working immediately.

The secret is printed to stdout exactly once and cannot be retrieved later.
Capture it directly into a secret store or .env file rather than a terminal
that is being recorded or shared.`,
	Args: namedProject("mint"),
	RunE: func(cmd *cobra.Command, args []string) error {
		label, _ := cmd.Flags().GetString("label")
		resp, err := apiClient.MintProjectKey(args[0], label)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "WARNING: mint ROTATES the project's key — any previous key no longer works.")
		if resp.Rotated {
			fmt.Fprintln(os.Stderr, "A previous active key was revoked by this mint.")
		}
		fmt.Fprintln(os.Stderr, "The secret below is shown once; store it now.")

		f := getFormatter()
		if !isTableOutput() {
			fmt.Print(f.FormatRaw(resp))
			return nil
		}
		fmt.Print(f.FormatSingle([]output.KeyValue{
			{Key: "Project", Value: resp.Project},
			{Key: "Label", Value: resp.Label},
			{Key: "Rotated", Value: fmt.Sprintf("%v", resp.Rotated)},
			{Key: "Secret", Value: resp.Secret},
		}))
		return nil
	},
}

var projectKeyInfoCmd = &cobra.Command{
	Use:   "info <project>",
	Short: "Show the active key's metadata (never the secret)",
	Args:  namedProject("info"),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := apiClient.GetProjectKey(args[0])
		if err != nil {
			return err
		}
		f := getFormatter()
		if !isTableOutput() {
			fmt.Print(f.FormatRaw(resp))
			return nil
		}
		if !resp.Exists {
			fmt.Printf("No active API key for project %q.\n", args[0])
			return nil
		}
		fields := []output.KeyValue{{Key: "Project", Value: args[0]}}
		if resp.Label != nil {
			fields = append(fields, output.KeyValue{Key: "Label", Value: *resp.Label})
		}
		if resp.CreatedAt != nil {
			fields = append(fields, output.KeyValue{Key: "Created", Value: resp.CreatedAt.Local().Format("2006-01-02 15:04:05")})
		}
		last := "never"
		if resp.LastUsedAt != nil {
			last = resp.LastUsedAt.Local().Format("2006-01-02 15:04:05")
		}
		fields = append(fields, output.KeyValue{Key: "Last used", Value: last})
		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

var projectKeyRevokeCmd = &cobra.Command{
	Use:   "revoke <project>",
	Short: "Revoke the project's active API key",
	Args:  namedProject("revoke"),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := apiClient.RevokeProjectKey(args[0]); err != nil {
			return err
		}
		// 204 No Content: there is no API object, so -o json gets a local
		// confirmation (same convention as `project delete`).
		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(map[string]string{"project": args[0], "status": "revoked"}))
			return nil
		}
		fmt.Printf("API key for project %q revoked.\n", args[0])
		return nil
	},
}

var projectKeyRequestCmd = &cobra.Command{
	Use:   "request <project>",
	Short: "Ask the project owner(s) to generate a key (team members)",
	Args:  namedProject("request"),
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := apiClient.RequestProjectKey(args[0])
		if err != nil {
			return err
		}
		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(resp))
			return nil
		}
		if resp.Notified {
			fmt.Printf("Project owner(s) notified of your request for %q.\n", args[0])
		} else {
			fmt.Printf("No owner could be emailed for %q; ask them directly.\n", args[0])
		}
		return nil
	},
}

func init() {
	projectKeyMintCmd.Flags().String("label", "default", "Name the key renders as in attribution")
	projectKeyCmd.AddCommand(projectKeyMintCmd, projectKeyInfoCmd, projectKeyRevokeCmd, projectKeyRequestCmd)
}

// namedProject answers a key command given no project with what to type, not
// cobra's arity message ("accepts 1 arg(s), received 0", Markus 2026-11-09).
// Keys are never resolved from a default project: mint rotates the live key.
func namedProject(verb string) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		switch {
		case len(args) == 0:
			return fmt.Errorf("which project? Name it: stompy project key %s <project>", verb)
		case len(args) > 1:
			return fmt.Errorf("one project at a time: stompy project key %s <project>", verb)
		}
		return nil
	}
}
