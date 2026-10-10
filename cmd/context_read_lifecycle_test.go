package cmd

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// STOMPY-2724: `context recall` and `context list` show a context's lifecycle
// state and review date (the facts MCP recall renders as its PARKED/ARCHIVED
// header), and -o json is the API object verbatim. These drive the REAL
// command tree (rootCmd.Execute) against an httptest server via runCLI.

func detailBody(topic, lifecycle, reviewAfter string) string {
	review := "null"
	if reviewAfter != "" {
		review = `"` + reviewAfter + `"`
	}
	return `{"id":7,"topic":"` + topic + `","version":"1.0","priority":"important","tags":["t"],
"content":"the body","access_count":3,"embedding_status":"completed",
"lifecycle":"` + lifecycle + `","review_after":` + review + `,"versions":[]}`
}

func listBody() string {
	return `{"contexts":[
{"id":1,"topic":"live_a","version":"1.0","priority":"important","tags":[],"access_count":1,"lifecycle":"active","review_after":null},
{"id":2,"topic":"sleep_b","version":"1.0","priority":"reference","tags":[],"access_count":2,"lifecycle":"parked","review_after":"2026-11-03"},
{"id":3,"topic":"reject_c","version":"2.0","priority":"reference","tags":[],"access_count":0,"lifecycle":"archived","review_after":null}
],"total":3,"limit":100,"offset":0,"by_priority":{"important":1,"reference":2}}`
}

func TestContextRecall_ShowsLifecycleState(t *testing.T) {
	cases := []struct {
		name        string
		lifecycle   string
		reviewAfter string
		want        []string // all must appear
		wantAbsent  []string
	}{
		{"active renders no state line", "active", "", nil, []string{"Lifecycle", "parked", "archived"}},
		{"parked names the review date", "parked", "2026-11-03", []string{"Lifecycle", "parked", "2026-11-03"}, nil},
		{"archived", "archived", "", []string{"Lifecycle", "archived"}, []string{"review"}},
		{"delivered", "delivered", "", []string{"Lifecycle", "delivered"}, []string{"review"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, reqs, err := runCLI(t, replyJSON(200, detailBody("sleep_b", tc.lifecycle, tc.reviewAfter)),
				"-p", "proj", "context", "recall", "sleep_b")
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if len(reqs) != 1 || reqs[0].path != "/projects/proj/contexts/sleep_b" {
				t.Fatalf("requests = %+v", reqs)
			}
			if !strings.Contains(out, "the body") {
				t.Errorf("content missing:\n%s", out)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output missing %q:\n%s", w, out)
				}
			}
			for _, w := range tc.wantAbsent {
				if strings.Contains(out, w) {
					t.Errorf("output must not contain %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestContextRecall_AnOlderServerWithoutTheFieldsStillRecalls(t *testing.T) {
	body := `{"id":7,"topic":"t","version":"1.0","priority":"reference","tags":[],"content":"the body","access_count":0}`
	out, _, err := runCLI(t, replyJSON(200, body), "-p", "proj", "context", "recall", "t")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(out, "the body") || strings.Contains(out, "Lifecycle") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestContextRecall_JSONIsTheAPIObject(t *testing.T) {
	out, _, err := runCLI(t, replyJSON(200, detailBody("sleep_b", "parked", "2026-11-03")),
		"-p", "proj", "-o", "json", "context", "recall", "sleep_b")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, out)
	}
	if got["lifecycle"] != "parked" || got["review_after"] != "2026-11-03" || got["topic"] != "sleep_b" {
		t.Errorf("not the API object: %v", got)
	}
	// embedding_status is a field the CLI struct does not model: it only
	// survives if the API object is passed through verbatim.
	if got["embedding_status"] != "completed" || got["content"] != "the body" {
		t.Errorf("fields dropped on the way through: %v", got)
	}
}

func TestContextList_ShowsLifecycleColumn(t *testing.T) {
	out, reqs, err := runCLI(t, replyJSON(200, listBody()), "-p", "proj", "context", "list")
	out = ansi.ReplaceAllString(out, "")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if len(reqs) != 1 || reqs[0].path != "/projects/proj/contexts" {
		t.Fatalf("requests = %+v", reqs)
	}
	lines := map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		for _, topic := range []string{"live_a", "sleep_b", "reject_c"} {
			if strings.Contains(l, topic) {
				lines[topic] = l
			}
		}
	}
	if !strings.Contains(out, "LIFECYCLE") || !strings.Contains(out, "REVIEW") {
		t.Fatalf("no LIFECYCLE column:\n%s", out)
	}
	for topic, want := range map[string]string{
		"live_a":   "active",
		"sleep_b":  "parked",
		"reject_c": "archived",
	} {
		if !strings.Contains(lines[topic], want) {
			t.Errorf("row %s = %q, want it to show %q", topic, lines[topic], want)
		}
	}
	if !strings.Contains(lines["sleep_b"], "2026-11-03") {
		t.Errorf("parked row must carry its review date: %q", lines["sleep_b"])
	}
	if strings.Contains(lines["reject_c"], "2026-") || strings.Contains(lines["live_a"], "2026-") {
		t.Errorf("only a parked row has a review date")
	}
}

func TestContextList_JSONIsTheAPIObject(t *testing.T) {
	out, _, err := runCLI(t, replyJSON(200, listBody()), "-p", "proj", "-o", "json", "context", "list")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, out)
	}
	rows, _ := got["contexts"].([]any)
	if len(rows) != 3 || got["total"] != float64(3) {
		t.Fatalf("not the API object: %v", got)
	}
	parked, _ := rows[1].(map[string]any)
	if parked["lifecycle"] != "parked" || parked["review_after"] != "2026-11-03" {
		t.Errorf("parked row = %v", parked)
	}
	if got["by_priority"] == nil {
		t.Errorf("by_priority (unmodeled) dropped: %v", got)
	}
}
