package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stompy-ai/stompy-cli/internal/api"
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

// STOMPY-2858: stompy-ticketing 0.8.14 adds context rows (kind="context").
const linksMixedKinds = `[
 {"kind":"ticket","id":10,"source_id":1,"target_id":2,"link_type":"blocks",
  "created_at":1792143289.62,
  "peer_id":2,"peer_title":"Beta blocked","peer_status":"backlog",
  "direction":"outgoing","relation":"blocks"},
 {"kind":"context","id":11,"ticket_id":1,"context_label":"release_failure_vectors",
  "context_version":"3","link_type":"relates_to","created_at":1792143300.5,
  "direction":"outgoing","relation":"relates_to",
  "source_id":null,"target_id":null,"peer_id":null,"peer_title":null,"peer_status":null}]`

func TestLinkListContextRowRendersLabelAtVersion(t *testing.T) {
	out := runLinkList(t, "1", linksMixedKinds, "table")
	var ctxLine string
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, "release_failure_vectors@3") {
			ctxLine = ln
		}
	}
	if ctxLine == "" {
		t.Fatalf("context row not rendered as label@version:\n%s", out)
	}
	for _, want := range []string{"11", "relates_to", "outgoing"} {
		if !strings.Contains(ctxLine, want) {
			t.Errorf("context row missing %q: %q", want, ctxLine)
		}
	}
	if strings.Contains(ctxLine, " 0 ") {
		t.Errorf("context row leaked a zero ticket id: %q", ctxLine)
	}
	if !strings.Contains(out, "Beta blocked") {
		t.Errorf("ticket row lost in mixed output:\n%s", out)
	}
}

func TestLinkListTicketRowUnchangedWithKindField(t *testing.T) {
	withKind := strings.Replace(linksNewShape, `{"id":10`, `{"kind":"ticket","created_at":1792143289.62,"id":10`, 1)
	if a, b := runLinkList(t, "1", withKind, "table"), runLinkList(t, "1", linksNewShape, "table"); a != b {
		t.Errorf("ticket row output changed by kind field:\n%s\nvs\n%s", a, b)
	}
}

func TestLinkListJSONPassesContextRowThrough(t *testing.T) {
	out := runLinkList(t, "1", linksMixedKinds, "json")
	for _, want := range []string{`"kind": "context"`, `"kind": "ticket"`, `"context_label": "release_failure_vectors"`, `"context_version": "3"`, `"ticket_id": 1`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %s in:\n%s", want, out)
		}
	}
}

func TestLinkListOldServerNoKindTreatedAsTicket(t *testing.T) {
	out := runLinkList(t, "1", linksNewShape, "table")
	if !strings.Contains(out, "Beta blocked") || strings.Contains(out, "@") {
		t.Errorf("no-kind row not treated as ticket:\n%s", out)
	}
}
