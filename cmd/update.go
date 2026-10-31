package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/output"
	"github.com/stompy-ai/stompy-cli/internal/update"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update stompy to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Current version: %s\n", Version)

		if err := update.SelfUpdate(Version); err != nil {
			return err
		}

		fmt.Printf("%s stompy has been updated.\n", output.Success("✓"))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
