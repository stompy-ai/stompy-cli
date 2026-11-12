package cmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stompy-ai/stompy-cli/internal/api"
)

// STOMPY-3551: the assignee filter existed only on MCP. `ticket list` had no
// --assignee flag, so an agent could not ask the CLI for its own tickets.
func TestTicketListSendsAssignee3551(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query().Get("assignee")
		w.Write([]byte(`{"columns":[],"total":0}`))
	}))
	defer srv.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	flag := ticketListCmd.Flags().Lookup("assignee")
	if flag == nil {
		t.Fatal("ticket list has no --assignee flag")
	}
	defer func() { _ = flag.Value.Set(""); flag.Changed = false }()
	if err := ticketListCmd.Flags().Set("assignee", "Astra"); err != nil {
		t.Fatal(err)
	}
	withOutputFormat("json", func() {
		captureStdout(t, func() {
			if err := ticketListCmd.RunE(ticketListCmd, nil); err != nil {
				t.Error(err)
			}
		})
	})
	if got != "Astra" {
		t.Fatalf("assignee sent = %q, want Astra", got)
	}
}
