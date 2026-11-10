package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// STOMPY-3522: positional-argument errors say what to type, from the command's
// own Use line ("missing <topic>: stompy context recall <topic>"), never cobra's
// raw "accepts 1 arg(s), received 0". A missing argument is never defaulted.

func arity(min, max int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		usage := strings.TrimSpace(strings.TrimPrefix(cmd.Use, cmd.Name()))
		show := strings.TrimSpace(cmd.CommandPath() + " " + usage)
		if len(args) < min {
			var required, named []string
			for _, tok := range strings.Fields(usage) {
				if strings.HasPrefix(tok, "-") || tok == "[flags]" {
					continue
				}
				named = append(named, tok)
				if !strings.HasPrefix(tok, "[") {
					required = append(required, tok)
				}
			}
			if len(required) < min { // the Use line brackets a required argument
				required = named
			}
			missing := "an argument"
			if len(args) < len(required) {
				missing = strings.Join(required[len(args):], " ")
			}
			return fmt.Errorf("missing %s: %s", missing, show)
		}
		if max >= 0 && len(args) > max {
			return fmt.Errorf("too many arguments (%d given): %s", len(args), show)
		}
		return nil
	}
}

func exactArgs(n int) cobra.PositionalArgs        { return arity(n, n) }
func minimumArgs(n int) cobra.PositionalArgs      { return arity(n, -1) }
func rangeArgs(min, max int) cobra.PositionalArgs { return arity(min, max) }
func maximumArgs(n int) cobra.PositionalArgs      { return arity(0, n) }
