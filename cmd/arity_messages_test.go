package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// STOMPY-3522: a missing or extra positional argument must say what to type,
// never cobra's raw "accepts N arg(s), received M". Walks every command, so a
// new one that reaches for cobra.ExactArgs fails here.
func walk(c *cobra.Command, visit func(*cobra.Command)) {
	visit(c)
	for _, child := range c.Commands() {
		walk(child, visit)
	}
}

func TestArityErrorsSayWhatToType(t *testing.T) {
	walk(rootCmd, func(c *cobra.Command) {
		if c.Args == nil {
			return
		}
		for _, args := range [][]string{{}, strings.Fields("a b c d e f g h i j")} {
			err := c.Args(c, args)
			if err == nil {
				continue
			}
			msg := err.Error()
			if strings.Contains(msg, "accepts") || strings.Contains(msg, "received") {
				t.Errorf("%s with %d args: raw cobra message %q", c.CommandPath(), len(args), msg)
			}
			if !strings.Contains(msg, c.CommandPath()) {
				t.Errorf("%s with %d args: %q does not show the command to type", c.CommandPath(), len(args), msg)
			}
		}
	})
}

func TestMissingArgumentNamesIt(t *testing.T) {
	c := &cobra.Command{Use: "recall <topic>", Args: exactArgs(1)}
	parent := &cobra.Command{Use: "context"}
	parent.AddCommand(c)
	err := c.Args(c, nil)
	if err == nil || err.Error() != "missing <topic>: context recall <topic>" {
		t.Fatalf("got %v", err)
	}
}
