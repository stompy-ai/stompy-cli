package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/banton/stompy-cli/internal/repomarker"
	"github.com/spf13/cobra"
)

// exitError carries a specific process exit code out of a command (Execute
// honours it). An empty msg means the command already printed its report.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// managedFiles are the agent-instruction files the managed block lives in.
var managedFiles = []string{"AGENTS.md", "CLAUDE.md"}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Pin this repo to Stompy project(s) via .stompy.yaml and agent instructions",
	Long: `Write .stompy.yaml (if absent) and generate/refresh the managed block
(<!-- stompy:begin --> ... <!-- stompy:end -->) in AGENTS.md and CLAUDE.md.

The block is generated from .stompy.yaml, so the two cannot drift. Text
outside the block is never touched. Re-running is a no-op.

  stompy init --project @owner/project --role primary --purpose "the product"
  stompy init            # refresh the blocks from the existing .stompy.yaml

Pass --project via the global -p/--project flag.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		role, _ := cmd.Flags().GetString("role")
		environment, _ := cmd.Flags().GetString("environment")
		purpose, _ := cmd.Flags().GetString("purpose")
		force, _ := cmd.Flags().GetBool("force")
		return runInit(".", flagProject, role, environment, purpose, force)
	},
}

func runInit(dir, project, role, environment, purpose string, force bool) error {
	markerPath := filepath.Join(dir, repomarker.FileName)
	existing, readErr := os.ReadFile(markerPath)
	exists := readErr == nil
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}

	var out []string
	switch {
	case project == "" && !exists:
		return fmt.Errorf("no %s here. Create one with:\n  stompy init --project @owner/project [--role primary|shared|reference] [--environment production|staging] [--purpose \"...\"]", repomarker.FileName)
	case project != "":
		m := &repomarker.Marker{
			Version:     1,
			Environment: environment,
			Projects:    []repomarker.Project{{Address: project, Role: role, Default: true, Purpose: purpose}},
		}
		data, err := m.Marshal()
		if err != nil {
			return err
		}
		if _, err := repomarker.Parse(data); err != nil {
			return err
		}
		if exists && !bytes.Equal(existing, data) {
			if !force {
				return fmt.Errorf("%s already exists and differs from what these flags would write; pass --force to overwrite it, or run `stompy init` with no flags to just refresh the managed blocks", repomarker.FileName)
			}
		}
		if !exists || !bytes.Equal(existing, data) {
			if err := os.WriteFile(markerPath, data, 0o644); err != nil {
				return err
			}
			out = append(out, "wrote "+repomarker.FileName)
		}
	}

	m, err := repomarker.Load(markerPath)
	if err != nil {
		return err
	}
	block := repomarker.RenderBlock(m)
	for _, name := range managedFiles {
		path := filepath.Join(dir, name)
		cur, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		next, err := repomarker.ApplyBlock(string(cur), block)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if next == string(cur) && err == nil && cur != nil {
			continue
		}
		if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
			return err
		}
		out = append(out, "updated "+name)
	}
	if len(out) == 0 {
		fmt.Println("already up to date")
	}
	for _, line := range out {
		fmt.Println(line)
	}
	fmt.Println("Note: project handles and names written to these files are visible to anyone who can read this repo; do not commit them to a public repo if they are private.")
	return nil
}

func init() {
	initCmd.Flags().String("role", repomarker.RolePrimary, "Role of the project: primary, shared, or reference (read-only)")
	initCmd.Flags().String("environment", repomarker.EnvProduction, "Environment: production or staging")
	initCmd.Flags().String("purpose", "", "One line saying what this project is for")
	initCmd.Flags().Bool("force", false, "Overwrite an existing .stompy.yaml")
	rootCmd.AddCommand(initCmd)
}
