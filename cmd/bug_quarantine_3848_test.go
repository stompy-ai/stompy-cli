package cmd

// STOMPY-3848: since backend STOMPY-3844 a bug list page keeps its valid rows and
// names each malformed row the caller may see in "quarantined". The CLI decoded only
// reports/total, so a reporter with a damaged report saw a silent gap. The note goes
// to stderr in every format, so `-o json` stays the exact row array scripts and the
// backend's pinned-CLI test parse.

import (
	"encoding/json"
	"strings"
	"testing"
)

const page3848 = `{"reports":[{"id":"BUG-2","title":"valid neighbor","status":"triage","severity":"low"}],` +
	`"total":2,"filters":{},"quarantined":[{"id":"BUG-1","reason":"malformed_report"}]}`

func TestBugListNamesQuarantinedRowsInTableOutput3848(t *testing.T) {
	stdout, stderr, _, err := runLifecycle(t, jsonReply(200, page3848), "-p", "foo", "bug", "list")
	if err != nil {
		t.Fatalf("bug list: %v", err)
	}
	if !strings.Contains(stdout, "BUG-2") {
		t.Fatalf("valid row missing from stdout: %q", stdout)
	}
	if !strings.Contains(stderr, "BUG-1") || !strings.Contains(stderr, "malformed_report") {
		t.Fatalf("quarantined row not named on stderr: %q", stderr)
	}
}

func TestBugListNamesQuarantinedRowsInJSONOutput3848(t *testing.T) {
	stdout, stderr, _, err := runLifecycle(t, jsonReply(200, page3848), "-o", "json", "-p", "foo", "bug", "list")
	if err != nil {
		t.Fatalf("bug list -o json: %v", err)
	}
	var rows []map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &rows); jerr != nil || len(rows) != 1 {
		t.Fatalf("stdout must stay the row array, got %q (%v)", stdout, jerr)
	}
	if !strings.Contains(stderr, "BUG-1") || !strings.Contains(stderr, "malformed_report") {
		t.Fatalf("quarantined row not named on stderr: %q", stderr)
	}
}

func TestControlBugListWithoutQuarantineSaysNothingExtra3848(t *testing.T) {
	clean := `{"reports":[{"id":"BUG-2","title":"valid","status":"triage","severity":"low"}],"total":1,"filters":{}}`
	_, stderr, _, err := runLifecycle(t, jsonReply(200, clean), "-o", "json", "-p", "foo", "bug", "list")
	if err != nil {
		t.Fatalf("bug list: %v", err)
	}
	if strings.Contains(stderr, "notice") || strings.Contains(stderr, "malformed") {
		t.Fatalf("no quarantine, yet stderr says %q", stderr)
	}
}
