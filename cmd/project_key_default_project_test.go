package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// Markus, 2026-11-09: `stompy project key info` with no argument answered
// cobra's raw "accepts 1 arg(s), received 0". Every key subcommand now says
// what to type instead, and none falls back to a default project.
func TestProjectKeyCommandsWithoutAProjectSayWhatToType(t *testing.T) {
	t.Setenv("STOMPY_PROJECT", "stompy")
	for _, sub := range []string{"mint", "info", "revoke", "request"} {
		_, reqs, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {}, "project", "key", sub)
		if err == nil {
			t.Errorf("%s with no project: no error, want a refusal", sub)
			continue
		}
		if !strings.Contains(err.Error(), "stompy project key "+sub+" <project>") {
			t.Errorf("%s: err = %q, want it to show the command to type", sub, err)
		}
		if strings.Contains(err.Error(), "accepts 1 arg(s)") {
			t.Errorf("%s: still cobra's arity message: %q", sub, err)
		}
		if len(reqs) != 0 {
			t.Errorf("%s: %d request(s) reached the server; a default project must never be used", sub, len(reqs))
		}
	}
}

func TestProjectKeyCommandsRefuseTwoProjects(t *testing.T) {
	for _, sub := range []string{"mint", "info", "revoke", "request"} {
		_, reqs, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {}, "project", "key", sub, "a", "b")
		if err == nil || !strings.Contains(err.Error(), "one project at a time: stompy project key "+sub+" <project>") {
			t.Errorf("%s: err = %v", sub, err)
		}
		if len(reqs) != 0 {
			t.Errorf("%s: %d request(s) reached the server", sub, len(reqs))
		}
	}
}
