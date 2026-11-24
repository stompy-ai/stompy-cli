package cmd

import (
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Cobra retains values and Changed bits across Execute calls, including flags
// on commands that the next call does not visit.
func resetLifecycleFlags(t *testing.T, command *cobra.Command) {
	t.Helper()
	reset := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(flag *pflag.Flag) {
			var err error
			if slice, ok := flag.Value.(pflag.SliceValue); ok {
				// Current list flags have empty defaults; Set("[]") appends a
				// literal item instead of clearing their retained contents.
				err = slice.Replace(nil)
			} else {
				err = flag.Value.Set(flag.DefValue)
			}
			if err != nil {
				t.Fatalf("reset %s --%s: %v", command.CommandPath(), flag.Name, err)
			}
			flag.Changed = false
		})
	}
	reset(command.Flags())
	reset(command.PersistentFlags())
	for _, child := range command.Commands() {
		resetLifecycleFlags(t, child)
	}
}

func drainLifecycleUpdate() {
	// Parsing and early pre-run errors never start the worker that closes this
	// channel. Bound the wait while still draining the normal dev-version check.
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case _, open := <-updateAvailable:
			if !open {
				return
			}
		case <-timeout:
			return
		}
	}
}
