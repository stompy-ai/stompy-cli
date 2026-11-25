package cmd

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestBug3865_SeverityComposesWithCanonicalFilters(t *testing.T) {
	for _, severity := range []string{"critical", "high", "medium", "low"} {
		t.Run(severity, func(t *testing.T) {
			out, requests, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer tok" {
					t.Errorf("ordinary reporter bearer lost: %q", r.Header.Get("Authorization"))
				}
				replyJSON(200, `{"reports":[{"id":"BUG-3865","title":"owned","status":"parked","severity":"`+severity+`"}],"total":1}`)(w, r)
			}, "-p", "foo", "-o", "json", "bug", "list", "--severity", severity,
				"--status", "parked", "--limit", "3", "--offset", "2")
			if err != nil {
				t.Fatalf("real command tree: %v", err)
			}
			if len(requests) != 1 || requests[0].method != "GET" || requests[0].path != "/bug-reports" {
				t.Fatalf("requests: %+v", requests)
			}
			want := map[string][]string{"severity": {severity}, "status": {"parked"}, "limit": {"3"}, "offset": {"2"}}
			if !reflect.DeepEqual(map[string][]string(requests[0].query), want) {
				t.Fatalf("filters or reporter scope changed: got %v, want %v", requests[0].query, want)
			}
			var rows []map[string]string
			if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 1 || rows[0]["ID"] != "BUG-3865" || rows[0]["SEVERITY"] != severity {
				t.Fatalf("filtered row array: %q (%v)", out, err)
			}
		})
	}
}

func TestBug3865_HelpNamesCanonicalStatuses(t *testing.T) {
	out, requests, err := runCLI(t, replyJSON(500, `{}`), "bug", "list", "--help")
	if err != nil || len(requests) != 0 {
		t.Fatalf("help must be local: %v %+v", err, requests)
	}
	if !strings.Contains(out, "triage, confirmed, in_progress, resolved, wont_fix, parked") {
		t.Fatalf("canonical status choices missing: %s", out)
	}
}

func TestBug3865_UnfilteredControl(t *testing.T) {
	out, requests, err := runCLI(t, replyJSON(200, `{"reports":[{"id":"BUG-1","title":"owned","status":"triage","severity":"low"}],"total":7}`), "-p", "foo", "bug", "list")
	if err != nil || len(requests) != 1 || len(requests[0].query) != 0 || !strings.Contains(out, "BUG-1") || !strings.Contains(out, "Total: 7 bug reports") {
		t.Fatalf("unfiltered control: %v %+v %q", err, requests, out)
	}
}

func TestBug3865_ReporterAuthHelpControl(t *testing.T) {
	if strings.Contains(bugCmd.Short, "X-API-Key") || strings.Contains(bugCmd.Short, "dev team only") || !strings.Contains(bugCmd.Short, "your bug reports") {
		t.Fatalf("ordinary reporter help regressed: %q", bugCmd.Short)
	}
}

func TestBug3865_RefusalControl(t *testing.T) {
	_, requests, err := runCLI(t, replyJSON(403, `{"detail":"forbidden"}`), "-p", "foo", "bug", "list")
	if err == nil || len(requests) != 1 || requests[0].path != "/bug-reports" {
		t.Fatalf("refusal must remain an error: %v %+v", err, requests)
	}
}
