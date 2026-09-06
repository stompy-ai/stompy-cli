package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// STOMPY-1967 item 1: the mutating commands (ticket create/update, context
// lock/unlock, project delete) used to print a human "✓ ..." line regardless
// of -o json, so a script could never read back the id/url/version the API
// returned. They must honour -o json with the API's own object, same as
// `project create` already does, and keep the ✓ line for table mode.

// captureStdout redirects os.Stdout for the duration of fn and returns what
// was written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// withOutputFormat sets flagOutput for the duration of fn and restores it.
func withOutputFormat(format string, fn func()) {
	old := flagOutput
	flagOutput = format
	defer func() { flagOutput = old }()
	fn()
}

func TestTicketCreate_HonoursJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(api.TicketResponse{
			ID: 1, Title: "New ticket", Type: "task", Status: "backlog", Priority: "medium",
			URL: "https://stompy.ai/dashboard/projects/proj/tickets/1",
		})
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	flagProject = "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	ticketCreateCmd.Flags().Set("title", "New ticket")

	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			if err := ticketCreateCmd.RunE(ticketCreateCmd, nil); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("-o json output is not valid JSON: %v\noutput: %q", err, out)
	}
	if got["url"] != "https://stompy.ai/dashboard/projects/proj/tickets/1" {
		t.Errorf("json output missing/wrong url: %+v", got)
	}
	if id, _ := got["id"].(float64); id != 1 {
		t.Errorf("json output missing/wrong id: %+v", got)
	}
}

func TestTicketCreate_TableModeKeepsCheckmark(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(api.TicketResponse{ID: 2, Title: "Table ticket", Type: "task", Status: "backlog", Priority: "medium"})
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	flagProject = "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	ticketCreateCmd.Flags().Set("title", "Table ticket")

	var out string
	withOutputFormat("table", func() {
		out = captureStdout(t, func() {
			if err := ticketCreateCmd.RunE(ticketCreateCmd, nil); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})

	if !strings.Contains(out, "Ticket #2 created") || !strings.Contains(out, "Table ticket") {
		t.Errorf("table output missing the ✓ line: %q", out)
	}
}

func TestTicketUpdate_HonoursJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.TicketResponse{
			ID: 1, Title: "Updated", Type: "task", Status: "backlog", Priority: "high",
			URL: "https://stompy.ai/dashboard/projects/proj/tickets/1",
		})
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	flagProject = "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	ticketUpdateCmd.Flags().Set("priority", "high")

	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			if err := ticketUpdateCmd.RunE(ticketUpdateCmd, []string{"1"}); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("-o json output is not valid JSON: %v\noutput: %q", err, out)
	}
	if got["priority"] != "high" {
		t.Errorf("json output missing/wrong priority: %+v", got)
	}
}

func TestContextLock_HonoursJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(api.ContextCreateResponse{
			ID: 7, Topic: "mytopic", Version: "1.0",
			URL: "https://stompy.ai/dashboard/projects/proj/contexts/mytopic@1.0",
		})
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	flagProject = "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	contextLockCmd.Flags().Set("content", "hello world")

	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			if err := contextLockCmd.RunE(contextLockCmd, []string{"mytopic"}); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("-o json output is not valid JSON: %v\noutput: %q", err, out)
	}
	if got["url"] == nil || got["url"] == "" {
		t.Errorf("json output missing url: %+v", got)
	}
	if got["version"] != "1.0" {
		t.Errorf("json output missing/wrong version: %+v", got)
	}
}

func TestContextUnlock_HonoursJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.ContextDeleteResponse{
			Topic: "mytopic", Archived: true, DeletedCount: 1,
			URL: "https://stompy.ai/dashboard/projects/proj/contexts/mytopic",
		})
	}))
	defer srv.Close()

	oldClient, oldProject := apiClient, flagProject
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	flagProject = "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()

	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			if err := contextUnlockCmd.RunE(contextUnlockCmd, []string{"mytopic"}); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("-o json output is not valid JSON: %v\noutput: %q", err, out)
	}
	if got["archived"] != true {
		t.Errorf("json output missing/wrong archived: %+v", got)
	}
}

func TestProjectDelete_HonoursJSONOutput(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	oldClient := apiClient
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	defer func() { apiClient = oldClient }()

	projectDeleteCmd.Flags().Set("confirm", "true")

	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			if err := projectDeleteCmd.RunE(projectDeleteCmd, []string{"myproj"}); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("-o json output is not valid JSON: %v\noutput: %q", err, out)
	}
	if got["name"] != "myproj" || got["status"] != "deleted" {
		t.Errorf("json output = %+v, want name=myproj status=deleted", got)
	}
}
