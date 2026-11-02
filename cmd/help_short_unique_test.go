package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// STOMPY-3370 item 3: `stompy ticket --help` listed archive, unarchive and
// batch-archive (and claim, release, claim-next) under one shared blurb.
// Sibling subcommands must each say what THEY do.
func TestHelp_SiblingSubcommandsHaveDistinctShorts(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		seen := map[string]string{}
		for _, sub := range c.Commands() {
			if sub.Hidden || sub.Short == "" {
				continue
			}
			if other, dup := seen[sub.Short]; dup {
				t.Errorf("%q: %q and %q share Short %q", c.CommandPath(), other, sub.Name(), sub.Short)
			}
			seen[sub.Short] = sub.Name()
			walk(sub)
		}
	}
	walk(rootCmd)
}
