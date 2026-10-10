package cmd

import (
	"strings"
	"testing"
)

// STOMPY-2663 follow-up: the server words `undo` for ITS door (MCP:
// set_lifecycle(...), REST: a POST route), and one string cannot be right for
// the CLI. The CLI prints its own command, whatever the server sent. Driven
// through the real command tree against an httptest server.
func TestConflictResolve_PrintsTheCLIUndoNotTheServers(t *testing.T) {
	const archivedMCP = `{"id":"c-1","status":"user_resolved","archived_context":{"archived":true,` +
		`"topic":"refund_window_b","version":"1.1","kept_topic":"refund_window_a",` +
		`"undo":"set_lifecycle(topic=\"refund_window_b\", state=\"active\")"}}`
	const archivedREST = `{"id":"c-1","status":"user_resolved","archived_context":{"archived":true,` +
		`"topic":"refund_window_b","version":"1.1","kept_topic":"refund_window_a",` +
		`"undo":"POST /api/v1/projects/proj/contexts/refund_window_b/lifecycle {\"state\": \"active\"}"}}`
	cases := []struct {
		name       string
		project    string
		resolution string
		reply      string
		wantUndo   string
		wantNoUndo bool
		wantAbsent []string
	}{
		{
			name: "keep_a, server sent the MCP form", project: "proj", resolution: "keep_a",
			reply:      archivedMCP,
			wantUndo:   "undo: stompy context lifecycle refund_window_b -p proj --state active",
			wantAbsent: []string{"set_lifecycle"},
		},
		{
			name: "keep_b, server sent the REST form", project: "proj", resolution: "keep_b",
			reply:      archivedREST,
			wantUndo:   "undo: stompy context lifecycle refund_window_b -p proj --state active",
			wantAbsent: []string{"POST ", "/api/v1"},
		},
		{
			name: "undo carries the project it ran in", project: "other-proj", resolution: "keep_a",
			reply:    archivedMCP,
			wantUndo: "undo: stompy context lifecycle refund_window_b -p other-proj --state active",
		},
		{
			name: "refused archive prints no undo", project: "proj", resolution: "keep_a",
			reply:      `{"id":"c-1","status":"user_resolved","archived_context":{"archived":false,"reason":"same_topic"}}`,
			wantNoUndo: true,
		},
		{
			name: "dismiss prints no undo", project: "proj", resolution: "dismiss",
			reply:      `{"id":"c-1","status":"dismissed"}`,
			wantNoUndo: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _, call, err := runLifecycle(t, jsonReply(200, tc.reply),
				"-p", tc.project, "conflict", "resolve", "c-1", "--resolution", tc.resolution)
			if err != nil {
				t.Fatalf("conflict resolve: %v", err)
			}
			if call.method != "POST" || call.path != "/conflicts/c-1/resolve" {
				t.Fatalf("request = %s %s", call.method, call.path)
			}
			if tc.wantNoUndo {
				if strings.Contains(out, "undo") {
					t.Errorf("output %q must not offer an undo", out)
				}
				return
			}
			if !strings.Contains(out, tc.wantUndo) {
				t.Errorf("output %q does not contain %q", out, tc.wantUndo)
			}
			for _, bad := range tc.wantAbsent {
				if strings.Contains(out, bad) {
					t.Errorf("output %q must not echo the server's door text %q", out, bad)
				}
			}
		})
	}
}
