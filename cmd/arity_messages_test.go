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

// Real shipped commands, exact wording (review of #57).
func TestRealCommandsNameWhatIsMissing(t *testing.T) {
	cases := []struct {
		path []string
		args []string
		want string
	}{
		{[]string{"context", "recall"}, nil, "missing <topic>: stompy context recall <topic>"},
		{[]string{"context", "batch"}, nil, "missing <topic1>: stompy context batch <topic1> <topic2> ..."},
		{[]string{"completion"}, nil, "missing bash|zsh|fish|powershell: stompy completion [bash|zsh|fish|powershell]"},
		{[]string{"completion"}, []string{"fish", "x"}, "too many arguments (2 given): stompy completion [bash|zsh|fish|powershell]"},
	}
	for _, c := range cases {
		cmd, _, err := rootCmd.Find(c.path)
		if err != nil {
			t.Fatalf("%v: %v", c.path, err)
		}
		got := cmd.Args(cmd, c.args)
		if got == nil || got.Error() != c.want {
			t.Errorf("%v %v: got %v, want %q", c.path, c.args, got, c.want)
		}
	}
}
