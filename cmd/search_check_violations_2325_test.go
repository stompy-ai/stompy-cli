package cmd

// STOMPY-2325: the CLI's third door for check_violations. `stompy search
// --check-violations` asks the REST route to name the rules the query would
// break, and shows the verdict in table output; -o json passes it through raw.

import (
	"net/http"
	"strings"
	"testing"
)

const verdict2325 = "Warning: Potential context violations detected:\n   - file_size_ratchet: Rewriting to accommodate GROWTH is refused."

func searchReply2325(seen *string, violations string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*seen = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		body := `{"results":[],"total_found":0,"search_mode":"hybrid"`
		switch violations {
		case "<absent>":
		case "":
			body += `,"violations":null,"violations_judged_by":"model"`
		case "<keywords>":
			body += `,"violations":null,"violations_judged_by":"keywords (the model's answer could not be read)"`
		default:
			body += `,"violations":` + jsonQuote(violations) + `,"violations_judged_by":"model"`
		}
		_, _ = w.Write([]byte(body + "}"))
	}
}

func jsonQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(s) + `"`
}

func TestSearchCheckViolationsAsksAndShowsTheRule2325(t *testing.T) {
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
	var seen string
	stdout, _, _, err := runLifecycle(t, searchReply2325(&seen, ""), "-p", "foo", "search", "anything")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if strings.Contains(seen, "check_violations") || strings.Contains(stdout, "Rule check") {
		t.Fatalf("flag off, yet query %q / output %q mention it", seen, stdout)
	}
}

func TestSearchCheckViolationsSaysWhenTheServerCannotCheck2325(t *testing.T) {
	var seen string
	stdout, _, _, err := runLifecycle(t, searchReply2325(&seen, "<absent>"),
		"-p", "foo", "search", "anything", "--check-violations")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(stdout, "not available from this server") || strings.Contains(stdout, "no rule broken") {
		t.Fatalf("an absent verdict must not read as a clean one: %q", stdout)
	}
}

func TestSearchCheckViolationsSaysWhenOnlyKeywordsJudged2325(t *testing.T) {
	var seen string
	stdout, _, _, err := runLifecycle(t, searchReply2325(&seen, "<keywords>"),
		"-p", "foo", "search", "merge without the RED harness", "--check-violations")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(stdout, "keyword check only") || !strings.Contains(stdout, "could not be read") {
		t.Fatalf("a keyword-only verdict must say so, not read as the model's all-clear: %q", stdout)
	}
}
