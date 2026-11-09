package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// STOMPY-3502: a title carrying OSC 52 (clipboard write) and a screen clear
// reaches stdout without them, on a door that prints through the formatter.
func TestATicketTitleCannotDriveTheTerminal3502(t *testing.T) {
	out, _, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":2,"title":"\u001b]52;c;ZXZpbA==\u0007\u001b[2JOrder the compactor","type":"task","status":"backlog","priority":"low"}`))
	}, "ticket", "get", "2", "-p", "p")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(out, "\x07\u009b\u009d") || strings.Contains(out, "\x1b]") || strings.Contains(out, "\x1b[2J") {
		t.Errorf("a server string drove the terminal:\n%q", out)
	}
	// The attempt stays visible (marked, not silently dropped).
	if !strings.Contains(out, `\x1b]52;c;`) {
		t.Errorf("the attempt is not visible:\n%q", out)
	}
	if !strings.Contains(out, "Order the compactor") {
		t.Errorf("the title itself was lost:\n%q", out)
	}
}
