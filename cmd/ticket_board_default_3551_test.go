package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stompy-ai/stompy-cli/internal/api"
)

// STOMPY-3551 item 5: `ticket board` printed column counts and no ticket rows,
// because the CLI defaulted to view=summary (counts only) while MCP and REST
// default to kanban. The CLI now shows the same board the other doors do.
func TestTicketBoardDefaultShowsRows3551(t *testing.T) {
	var view string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		view = r.URL.Query().Get("view")
		if view == "summary" {
			w.Write([]byte(`{"columns":[{"status":"backlog","count":1,"tickets":[]}],"total":1}`))
			return
		}
		w.Write([]byte(`{"columns":[{"status":"backlog","count":1,"tickets":[` +
			`{"id":7,"title":"Pair the card reader","type":"task","status":"backlog","priority":"high"}]}],"total":1}`))
	}))
	defer srv.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "proj"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	var out string
	withOutputFormat("table", func() {
		out = captureStdout(t, func() {
			if err := ticketBoardCmd.RunE(ticketBoardCmd, nil); err != nil {
				t.Error(err)
			}
		})
	})
	if view != "kanban" {
		t.Errorf("default view sent = %q, want kanban", view)
	}
	if !strings.Contains(out, "Pair the card reader") {
		t.Fatalf("board printed no ticket rows:\n%s", out)
	}
}
