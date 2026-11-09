package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// STOMPY-3500: `ticket get`, `move` and `close` show the ticket's links. The
// server answers links and context_links on all three since stompy-ticketing
// 0.8.32 / dementia #1636; the CLI decoded links and printed nothing, and never
// decoded context_links at all (dogfood 6.6.36 rounds 1-3).
const ticketWithLinks3500 = `{"id":2,"title":"Order the compactor","type":"task","status":"done","priority":"high",
 "links":[{"id":1,"source_id":2,"target_id":3,"link_type":"blocks","peer_id":3,"peer_title":"Lay the stone","peer_status":"backlog","direction":"outgoing","relation":"blocks"}],
 "context_links":[{"id":7,"ticket_id":2,"context_label":"compactor_hire_terms","context_version":"latest","link_type":"implements"},"legacy-string-row"]}`

func TestTicketCommandsShowTheirLinks3500(t *testing.T) {
	for _, args := range [][]string{
		{"ticket", "get", "2", "-p", "p"},
		{"ticket", "move", "2", "--status", "done", "-p", "p"},
		{"ticket", "close", "2", "-p", "p"},
	} {
		out, _, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(ticketWithLinks3500))
		}, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		for _, want := range []string{"blocks", "#3", "Lay the stone", "implements", "compactor_hire_terms"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: output lacks %q:\n%s", args, want, out)
			}
		}
	}
}

func TestATicketWithoutLinksPrintsNoLinksBlock3500(t *testing.T) {
	out, _, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":5,"title":"t","type":"task","status":"done","priority":"low","links":[],"context_links":[]}`))
	}, "ticket", "close", "5", "-p", "p")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "link:") || strings.Contains(out, "Links") {
		t.Errorf("unlinked ticket printed a links block:\n%s", out)
	}
}

// Review of #50: a context link the server lists in both arrays prints once,
// and an unversioned one prints no invented version.
func TestAContextLinkInBothArraysPrintsOnce3500(t *testing.T) {
	body := `{"id":2,"title":"t","type":"task","status":"done","priority":"low",
 "links":[{"id":7,"kind":"context","ticket_id":2,"context_label":"terms","link_type":"implements","relation":"implements"}],
 "context_links":[{"id":7,"ticket_id":2,"context_label":"terms","context_version":"","link_type":"implements"},{}]}`
	out, _, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}, "ticket", "close", "2", "-p", "p")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "link:"); n != 1 {
		t.Errorf("want exactly one link line, got %d:\n%s", n, out)
	}
	if strings.Contains(out, "terms@") {
		t.Errorf("unversioned link printed a version:\n%s", out)
	}
}
