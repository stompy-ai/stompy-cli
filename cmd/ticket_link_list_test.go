package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// STOMPY-2847: stompy-ticketing 0.8.13 link rows carry viewer-relative
// peer_* fields. The old shape paired TARGET ID=<other> with the viewed
// ticket's own title.
const linksNewShape = `[
 {"id":10,"source_id":1,"target_id":2,"link_type":"blocks",
  "source_title":"Alpha blocker","source_status":"in_progress",
  "target_title":"Beta blocked","target_status":"backlog",
  "peer_id":2,"peer_title":"Beta blocked","peer_status":"backlog",
  "direction":"outgoing","relation":"blocks"}]`

const linksNewShapeIncoming = `[
 {"id":10,"source_id":1,"target_id":2,"link_type":"blocks",
  "source_title":"Alpha blocker","source_status":"in_progress",
  "target_title":"Beta blocked","target_status":"backlog",
  "peer_id":1,"peer_title":"Alpha blocker","peer_status":"in_progress",
  "direction":"incoming","relation":"blocked_by"}]`

// Old backend, viewed from the target end: raw target_id is the viewed
// ticket itself, target_title is not trustworthy for the counterpart.
const linksOldShapeReverse = `[
 {"id":10,"source_id":1,"target_id":2,"link_type":"blocks",
  "target_title":"Beta blocked","target_status":"backlog"}]`

const linksOldShapeForward = `[
 {"id":10,"source_id":1,"target_id":2,"link_type":"blocks",
  "target_title":"Beta blocked","target_status":"backlog"}]`

func runLinkList(t *testing.T, viewed, body, format string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	oc, op, of := apiClient, flagProject, flagOutput
	apiClient, flagProject, flagOutput = api.NewClient(srv.URL, "tok", "dev", false), "mine", format
	defer func() { apiClient, flagProject, flagOutput = oc, op, of }()
	var err error
	out := captureStdout(t, func() { err = ticketLinkListCmd.RunE(ticketLinkListCmd, []string{viewed}) })
	if err != nil {
		t.Fatalf("link list: %v", err)
	}
	return out
}

func TestLinkListNewShapeOutgoingRendersPeer(t *testing.T) {
	out := runLinkList(t, "1", linksNewShape, "table")
	for _, want := range []string{"RELATION", "TICKET", "TITLE", "Beta blocked", "backlog", "blocks"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Alpha blocker") {
		t.Errorf("viewed ticket's own title leaked into peer column:\n%s", out)
	}
}

func TestLinkListNewShapeIncomingRendersPeer(t *testing.T) {
	out := runLinkList(t, "2", linksNewShapeIncoming, "table")
	for _, want := range []string{"blocked_by", "Alpha blocker", "in_progress"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Beta blocked") {
		t.Errorf("viewed ticket's own title leaked:\n%s", out)
	}
}

func TestLinkListOldShapeReverseDoesNotPairMismatchedIDAndTitle(t *testing.T) {
	out := runLinkList(t, "2", linksOldShapeReverse, "table")
	if strings.Contains(out, "Beta blocked") {
		t.Errorf("old-shape reverse row printed an untrusted title next to the other id:\n%s", out)
	}
	if !strings.Contains(out, "1") {
		t.Errorf("counterpart id missing:\n%s", out)
	}
}

func TestLinkListOldShapeForwardKeepsTitle(t *testing.T) {
	out := runLinkList(t, "1", linksOldShapeForward, "table")
	if !strings.Contains(out, "Beta blocked") || !strings.Contains(out, "backlog") {
		t.Errorf("old-shape forward row lost target title/status:\n%s", out)
	}
}

func TestLinkListJSONIncludesPeerFields(t *testing.T) {
	out := runLinkList(t, "2", linksNewShapeIncoming, "json")
	for _, want := range []string{`"direction": "incoming"`, `"relation": "blocked_by"`, `"peer_id"`, `"peer_title": "Alpha blocker"`, `"peer_status": "in_progress"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %s in:\n%s", want, out)
		}
	}
}
