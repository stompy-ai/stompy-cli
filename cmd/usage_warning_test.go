package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// captureStderr redirects os.Stderr for the duration of fn and returns what
// was written to it.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// STOMPY-2232: a create that crosses the threshold prints the usage line on
// stderr and leaves -o json stdout exactly as the plain create would — a
// script piping `stompy ticket create -o json | jq .id` must never see the
// sibling or the line.
func TestTicketCreate_UsageWarning_StderrOnly_StdoutUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":1,"title":"New ticket","type":"task","status":"backlog","priority":"medium",`+
			`"url":"https://stompy.ai/dashboard/projects/proj/tickets/1",`+
			`"usage_warning":{"used":241,"cap":300,"pct":80,"resets_at":"2026-10-01T00:00:00+00:00","upgrade_url":"https://www.stompy.ai/dashboard/settings"}}`)
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	if err := ticketCreateCmd.Flags().Set("title", "New ticket"); err != nil {
		t.Fatal(err)
	}

	var out, errOut string
	var runErr error
	withOutputFormat("json", func() {
		errOut = captureStderr(t, func() {
			out = captureStdout(t, func() {
				runErr = ticketCreateCmd.RunE(ticketCreateCmd, nil)
			})
		})
	})
	if runErr != nil {
		t.Fatalf("RunE() error: %v", runErr)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("-o json output is not valid JSON: %v\noutput: %q", err, out)
	}
	if _, present := got["usage_warning"]; present {
		t.Errorf("usage_warning leaked into stdout JSON: %v", got)
	}
	if got["id"] != float64(1) {
		t.Errorf("stdout missing the real payload: %+v", got)
	}

	if !strings.HasPrefix(errOut, "Usage: 241 of 300 units this month — resets 1 Oct.") {
		t.Fatalf("stderr = %q", errOut)
	}
}

func TestTicketCreate_NoUsageWarning_SilentStderr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":2,"title":"Quiet ticket","type":"task","status":"backlog","priority":"medium"}`)
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	if err := ticketCreateCmd.Flags().Set("title", "Quiet ticket"); err != nil {
		t.Fatal(err)
	}

	var runErr error
	errOut := captureStderr(t, func() {
		captureStdout(t, func() {
			runErr = ticketCreateCmd.RunE(ticketCreateCmd, nil)
		})
	})
	if runErr != nil {
		t.Fatalf("RunE() error: %v", runErr)
	}
	if errOut != "" {
		t.Fatalf("stderr = %q, want empty below the threshold", errOut)
	}
}
