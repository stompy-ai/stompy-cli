package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/spf13/cobra"
)

// STOMPY-2769: golden-shape contract for every `-o json` output that
// dementia-production's scripts/cli_battery_smoke.sh (and
// scripts/lib/cli_json_shapes.sh) parse with jq. stompy-cli #27 changed
// `context search -o json` from an array of {"TOPIC":..} to the hybrid
// route's {"results":[{"label":..}],...} object and only the server repo's
// Staging Daily Battery noticed, a day later. This table makes OUR CI notice.
//
// A failure here means the printed shape changed. If that is deliberate, edit
// BOTH sides in the same breath: this table AND the jq in
// scripts/cli_battery_smoke.sh (dementia-production), then say so in the PR.

const batteryConsumer = "scripts/cli_battery_smoke.sh (dementia-production)"

// shape is what a consumer relies on: the top-level JSON type, the keys (and
// their JSON types) on that top-level object, or on each array item.
type shape struct {
	top      string            // "array" | "object"
	keys     map[string]string // object: key -> json type
	itemKeys map[string]string // array: key -> json type, checked on item 0
	arrayAt  string            // object: key of an array whose item 0 is checked against itemKeys
}

func jsonType(v any) string {
	switch v.(type) {
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case nil:
		return "null"
	}
	return "unknown"
}

func checkKeys(obj map[string]any, want map[string]string) []string {
	var bad []string
	for k, typ := range want {
		v, ok := obj[k]
		switch {
		case !ok:
			bad = append(bad, fmt.Sprintf("key %q is missing", k))
		case jsonType(v) != typ:
			bad = append(bad, fmt.Sprintf("key %q is %s, want %s", k, jsonType(v), typ))
		}
	}
	return bad
}

func (s shape) violations(out string) []string {
	var v any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return []string{"output is not valid JSON: " + err.Error()}
	}
	if got := jsonType(v); got != s.top {
		return []string{fmt.Sprintf("top-level JSON is %s, want %s", got, s.top)}
	}
	var bad []string
	switch s.top {
	case "object":
		obj := v.(map[string]any)
		bad = append(bad, checkKeys(obj, s.keys)...)
		if s.arrayAt != "" {
			arr, _ := obj[s.arrayAt].([]any)
			if len(arr) == 0 {
				return append(bad, fmt.Sprintf("%q is empty or not an array; the fixture has one item", s.arrayAt))
			}
			item, _ := arr[0].(map[string]any)
			for _, b := range checkKeys(item, s.itemKeys) {
				bad = append(bad, s.arrayAt+"[0]: "+b)
			}
		}
	case "array":
		arr := v.([]any)
		if s.itemKeys != nil {
			if len(arr) == 0 {
				return []string{"array is empty; the fixture has one item"}
			}
			item, ok := arr[0].(map[string]any)
			if !ok {
				return []string{fmt.Sprintf("item 0 is %s, want object", jsonType(arr[0]))}
			}
			bad = append(bad, checkKeys(item, s.itemKeys)...)
		}
	}
	return bad
}

// runJSON serves body for any request, runs the command with -o json against
// project "proj" and returns what it printed.
func runJSON(t *testing.T, c *cobra.Command, args []string, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	flagProject = "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			if err := c.RunE(c, args); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})
	return out
}

func TestJSONOutputShapes_BatteryContract(t *testing.T) {
	str := "string"
	cases := []struct {
		name string // the exact command line the battery runs
		cmd  *cobra.Command
		args []string
		body string // the live route's response, trimmed to what matters
		want shape
		why  string // the jq the battery runs on it
	}{
		{
			name: "project list -o json",
			cmd:  projectListCmd,
			body: `{"projects":[{"name":"proj","schema_name":"s","created_at":"2026-01-02T03:04:05Z","role":"owner","is_system":false}],"total":1}`,
			want: shape{top: "array", itemKeys: map[string]string{"NAME": str}},
			why:  `any(.[]; .NAME == $p)`,
		},
		{
			name: "project info <p> -o json",
			cmd:  projectInfoCmd,
			args: []string{"proj"},
			body: `{"name":"proj","schema_name":"s","created_at":"2026-01-02T03:04:05Z","role":"owner","is_system":false}`,
			want: shape{top: "object", keys: map[string]string{"Name": str}},
			why:  `.Name`,
		},
		{
			name: "context recall <topic> -p proj -o json",
			cmd:  contextRecallCmd,
			args: []string{"topic_a"},
			body: `{"id":1,"topic":"topic_a","version":"1.0","priority":"reference","tags":[],"access_count":0,"content":"battery smoke content"}`,
			want: shape{top: "object", keys: map[string]string{"Content": str}},
			why:  `.Content`,
		},
		{
			name: "context search <q> -p proj -o json",
			cmd:  contextSearchCmd,
			args: []string{"battery smoke"},
			body: `{"results":[{"id":1,"label":"topic_a","preview":"p","metadata":"{}","source":"bm25","scope":"project"}],"total_found":1,"search_mode":"bm25_only"}`,
			want: shape{top: "object", keys: map[string]string{"results": "array"}, arrayAt: "results", itemKeys: map[string]string{"label": str}},
			why:  `context_search_labels (scripts/lib/cli_json_shapes.sh): .results[].label`,
		},
		{
			name: "conflict list -p proj -o json",
			cmd:  conflictListCmd,
			body: `{"conflicts":[],"total":0,"pending_count":0,"resolved_count":0}`,
			want: shape{top: "array"},
			why:  `type == "array"`,
		},
		{
			name: "ticket get <id> -p proj -o json",
			cmd:  ticketGetCmd,
			args: []string{"7"},
			body: `{"id":7,"title":"T","type":"task","status":"in_progress","priority":"medium"}`,
			want: shape{top: "object", keys: map[string]string{"Status": str}},
			why:  `.Status`,
		},
		{
			name: "ticket search <q> -p proj -o json",
			cmd:  ticketSearchCmd,
			args: []string{"battery"},
			body: `{"tickets":[{"id":7,"title":"T","type":"task","status":"backlog","priority":"medium"}],"total":1,"query":"battery"}`,
			want: shape{top: "array", itemKeys: map[string]string{"ID": str}},
			why:  `any(.[]; .ID == $id)  ($id is a string)`,
		},
		{
			name: "bug list -p proj -o json",
			cmd:  bugListCmd,
			body: `{"reports":[],"total":0}`,
			want: shape{top: "array"},
			why:  `type == "array"`,
		},
	}

	for _, tc := range cases {
		t.Run(strings.ReplaceAll(tc.name, " ", "_"), func(t *testing.T) {
			out := runJSON(t, tc.cmd, tc.args, tc.body)
			if bad := tc.want.violations(out); len(bad) > 0 {
				t.Errorf("`stompy %s` JSON shape changed: %s\n"+
					"  consumer: %s reads it with jq: %s\n"+
					"  if this change is deliberate, edit BOTH this table (cmd/json_shape_contract_test.go) "+
					"and that script in the same change.\n  output: %s",
					tc.name, strings.Join(bad, "; "), batteryConsumer, tc.why, out)
			}
		})
	}
}
