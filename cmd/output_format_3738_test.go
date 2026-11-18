package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test3738CheckNormalizesJSONAndKeepsQualifiedMarker(t *testing.T) {
	for _, format := range []string{"JSON", "Json", " json "} {
		for _, suffix := range []bool{false, true} {
			t.Run(format+map[bool]string{false: "_prefix", true: "_suffix"}[suffix], func(t *testing.T) {
				dir := repoDir(t, marker3709)
				args := []string{"-o", format, "project", "check"}
				if suffix {
					args = []string{"project", "check", "-o", format}
				}
				out, requests, err := runCLI(t, alias3709, args...)
				var rows []checkRow
				if err != nil || json.Unmarshal([]byte(out), &rows) != nil || len(rows) != 1 {
					t.Fatalf("not a successful JSON answer: %v %s", err, out)
				}
				if rows[0].Status != "renamed" || rows[0].NowCalled != "@alice/current" || rows[0].MarkerLine != `  - address: "@alice/current"` {
					t.Fatalf("lost qualified correction: %+v", rows)
				}
				if len(requests) != 1 || requests[0].query.Get("owner") != "alice" {
					t.Fatalf("lost owner: %+v", requests)
				}
				stored, err := os.ReadFile(filepath.Join(dir, ".stompy.yaml"))
				if err != nil || string(stored) != marker3709 {
					t.Fatalf("marker changed: %v %s", err, stored)
				}
			})
		}
	}
}

func Test3738UnknownOutputRefusedBeforeCommandOrNetwork(t *testing.T) {
	for _, command := range [][]string{{"project", "check"}, {"version"}} {
		repoDir(t, marker3709)
		_, requests, err := runCLI(t, alias3709, append(command, "-o", "wide")...)
		if exitCode(err) != 2 || !strings.Contains(err.Error(), `unknown output format "wide": use table, json or yaml`) || len(requests) != 0 {
			t.Fatalf("invalid format reached command: %v %+v", err, requests)
		}
	}
}
