package cmd

import (
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/contract"
)

// STOMPY-2728, the oracle. surface_contract.json maps COMMAND NAMES to
// capabilities; nothing asserted which ROUTE a command calls, so
// `context search` shipped calling the list route's lexical filter and
// returning [] for a query the other two doors find.
//
// This walks internal/contract/command_routes.json — every command the
// backend contract maps to a REST route that was verified — executes it
// through the real command tree, and asserts the FIRST request it issues is
// exactly the contract's method + path. A new route-bearing command that
// lands without a row here is invisible to this test: add it to the table.
func TestCommandRoutesOracle(t *testing.T) {
	table, err := contract.Load()
	if err != nil {
		t.Fatalf("command_routes.json: %v", err)
	}
	if len(table.Routes) == 0 {
		t.Fatal("command_routes.json lists no routes")
	}
	// Concrete values the test's args use for the contract's placeholders.
	fill := strings.NewReplacer(
		"{name}", "proj", "{project_name}", "proj", "{topic}", "topic_a",
		"{ticket_id}", "42", "{conflict_id}", "c-1",
	)
	for _, e := range table.Routes {
		t.Run(strings.ReplaceAll(e.Command, " ", "_"), func(t *testing.T) {
			if c, _, ferr := rootCmd.Find(strings.Fields(e.Command)); ferr != nil || c == nil || c.Name() != strings.Fields(e.Command)[len(strings.Fields(e.Command))-1] {
				t.Fatalf("command %q (%s) is not in the command tree", e.Command, e.Capability)
			}
			_, reqs, _ := runCLI(t, replyJSON(200, `{}`), e.Args...)
			if len(reqs) == 0 {
				t.Fatalf("`stompy %s` issued no request", strings.Join(e.Args, " "))
			}
			want := e.Method + " " + fill.Replace(e.PathTemplate)
			got := reqs[0].method + " " + reqs[0].path
			if got != want {
				t.Errorf("%s (%s): `stompy %s` called %s, contract says %s",
					e.Command, e.Capability, strings.Join(e.Args, " "), got, want)
			}
		})
	}
}
