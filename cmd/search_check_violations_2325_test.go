package cmd

// STOMPY-2325: the CLI's third door for check_violations. `stompy search
// --check-violations` asks the REST route to name the rules the query would
// break, and shows the verdict in table output; -o json passes it through raw.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// resetSearchFlags2325: cobra keeps flag values across Execute calls in one
// process, and runLifecycle resets only the root and lifecycle flags.
func resetSearchFlags2325(t *testing.T) {
	t.Helper()
	if c, _, err := rootCmd.Find([]string{"search"}); err == nil && c != nil {
		c.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	}
}

const verdict2325 = "Warning: Potential context violations detected:\n   - file_size_ratchet: Rewriting to accommodate GROWTH is refused."

func searchReply2325(seen *string, violations string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*seen = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		body := `{"results":[],"total_found":0,"search_mode":"hybrid"`
		if violations != "" {
			body += `,"violations":` + jsonQuote(violations)
		}
		_, _ = w.Write([]byte(body + "}"))
	}
}

func jsonQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}

func TestSearchCheckViolationsAsksAndShowsTheRule2325(t *testing.T) {
	resetSearchFlags2325(t)
	var seen string
	stdout, _, _, err := runLifecycle(t, searchReply2325(&seen, verdict2325),
		"-p", "foo", "search", "raise the baseline", "--check-violations")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(seen, "check_violations=true") {
		t.Fatalf("query %q lacks check_violations=true", seen)
	}
	if !strings.Contains(stdout, "file_size_ratchet") || !strings.Contains(stdout, "GROWTH is refused") {
		t.Fatalf("verdict not shown: %q", stdout)
	}
}

func TestSearchCheckViolationsSaysWhenNoRuleIsBroken2325(t *testing.T) {
	resetSearchFlags2325(t)
	var seen string
	stdout, _, _, err := runLifecycle(t, searchReply2325(&seen, ""),
		"-p", "foo", "search", "write a test first", "--check-violations")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(stdout, "no rule broken") {
		t.Fatalf("a null verdict must be stated, not omitted: %q", stdout)
	}
}

func TestControlSearchWithoutTheFlagSendsNothing2325(t *testing.T) {
	resetSearchFlags2325(t)
	var seen string
	stdout, _, _, err := runLifecycle(t, searchReply2325(&seen, ""), "-p", "foo", "search", "anything")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if strings.Contains(seen, "check_violations") || strings.Contains(stdout, "Rule check") {
		t.Fatalf("flag off, yet query %q / output %q mention it", seen, stdout)
	}
}
