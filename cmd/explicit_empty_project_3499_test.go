package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// STOMPY-3499 wiring: an explicit `-p ""` on the real command tree is refused
// before any request, even with a default project available. Reverting
// getProject's Changed("project") plumbing turns this red.
func TestExplicitEmptyProjectFlagNeverReachesTheServer3499(t *testing.T) {
	for _, args := range [][]string{
		{"context", "list", "-p", ""},
		{"ticket", "list", "-p", "  "},
	} {
		t.Setenv("STOMPY_PROJECT", "somewhere")
		_, reqs, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"contexts":[],"tickets":[]}`))
		}, args...)
		if err == nil || !strings.Contains(err.Error(), "project name is empty") {
			t.Errorf("%v: err = %v, want the empty-name refusal", args, err)
		}
		if len(reqs) != 0 {
			t.Errorf("%v: %d request(s) reached the server, first %s %s", args, len(reqs), reqs[0].method, reqs[0].path)
		}
	}
}
