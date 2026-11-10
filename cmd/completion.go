package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for stompy.

To load completions:

Bash:
  $ source <(stompy completion bash)
  # To load completions for each session, execute once:
  $ stompy completion bash > /etc/bash_completion.d/stompy

Zsh:
  $ source <(stompy completion zsh)
  # To load completions for each session, execute once:
  $ stompy completion zsh > "${fpath[1]}/_stompy"

Fish:
  $ stompy completion fish | source
  # To load completions for each session, execute once:
  $ stompy completion fish > ~/.config/fish/completions/stompy.fish

PowerShell:
  PS> stompy completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(exactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
