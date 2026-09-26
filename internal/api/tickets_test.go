package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// STOMPY-1967 item 6: GET /projects/{name}/tickets answers the kanban BOARD
// shape ({"columns":[{"status","count","tickets":[...]}],"total":N}), not a
// flat {"tickets":[...]}. ListTickets must flatten columns -> a ticket list.
func TestListTickets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/projects/proj/tickets" {
			t.Errorf("path = %s, want /projects/proj/tickets", r.URL.Path)
		}
		if r.URL.Query().Get("status") != "open" {
			t.Errorf("status = %q, want open", r.URL.Query().Get("status"))
		}
		w.Write([]byte(`{"columns":[{"status":"open","count":1,"tickets":[` +
			`{"id":1,"title":"Fix bug","type":"bug","status":"open","priority":"high"}` +
			`]}],"total":1}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListTickets("proj", "open", "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListTickets() error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if len(resp.Tickets) != 1 {
		t.Fatalf("len(Tickets) = %d, want 1 (columns were not flattened)", len(resp.Tickets))
	}
	if resp.Tickets[0].Title != "Fix bug" {
		t.Errorf("Title = %q, want %q", resp.Tickets[0].Title, "Fix bug")
	}
}

// TestListTickets_LiveBoardFixture decodes a RECORDED live response
// (staging 2026-09-06, identity user 51, project dogfood_cli_1967) to guard
// against the next route reshape going unnoticed.
func TestListTickets_LiveBoardFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/ticket_list_board.json")
	if err != nil {
		t.Fatal(err)
	}
	var resp TicketListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if len(resp.Tickets) != 1 {
		t.Fatalf("len(Tickets) = %d, want 1", len(resp.Tickets))
	}
	got := resp.Tickets[0]
	if got.ID != 1 || got.Title != "test ticket for 1967" || got.Status != "backlog" {
		t.Errorf("Tickets[0] = %+v, want id=1 title=%q status=backlog", got, "test ticket for 1967")
	}
	if got.URL == "" {
		t.Error("expected URL to be populated from the live fixture")
	}
}

// TestListTickets_FlatShapeFallback: /tickets/search answers a flat
// {"tickets":[...]} shape (no columns) — TicketListResponse stays reusable
// for that case too.
func TestListTickets_FlatShapeFallback(t *testing.T) {
	body := []byte(`{"tickets":[{"id":9,"title":"Flat","type":"task","status":"done","priority":"low"}],"total":1}`)
	var resp TicketListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if len(resp.Tickets) != 1 || resp.Tickets[0].Title != "Flat" {
		t.Errorf("Tickets = %+v, want one ticket titled Flat", resp.Tickets)
	}
}

// STOMPY-2526: GET /projects/{name}/tickets has no `status` query param —
// list_project_tickets (src/api/routes/tickets.py, read via origin/main)
// only accepts include_terminal/type_filter/limit, groups tickets into board
// columns, and drops terminal statuses unless include_terminal=true. An
// explicit --status must therefore (a) ask the server to include terminal
// columns and (b) narrow the flattened result to that status client-side —
// otherwise the flag is silently ignored, which is the dogfood repro.
func TestListTickets_StatusFilterHonored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("include_terminal") != "true" {
			t.Errorf("include_terminal = %q, want true (server hides terminal statuses unless asked)", r.URL.Query().Get("include_terminal"))
		}
		w.Write([]byte(`{"columns":[` +
			`{"status":"backlog","count":1,"tickets":[{"id":1,"title":"Backlog item","type":"task","status":"backlog","priority":"low"}]},` +
			`{"status":"resolved","count":1,"tickets":[{"id":2,"title":"Fixed bug","type":"bug","status":"resolved","priority":"high"}]}` +
			`],"total":2}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListTickets("proj", "resolved", "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListTickets() error: %v", err)
	}
	if len(resp.Tickets) != 1 {
		t.Fatalf("len(Tickets) = %d, want 1 (--status resolved must exclude the backlog ticket)", len(resp.Tickets))
	}
	if resp.Tickets[0].Status != "resolved" {
		t.Errorf("Tickets[0].Status = %q, want resolved", resp.Tickets[0].Status)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1 (matches the filtered count, not the unfiltered board total)", resp.Total)
	}
}

func TestGetTicket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/tickets/42" {
			t.Errorf("path = %s, want /projects/proj/tickets/42", r.URL.Path)
		}
		json.NewEncoder(w).Encode(TicketResponse{
			ID: 42, Title: "Implement feature", Type: "feature", Status: "open", Priority: "medium",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetTicket("proj", 42)
	if err != nil {
		t.Fatalf("GetTicket() error: %v", err)
	}
	if resp.ID != 42 {
		t.Errorf("ID = %d, want 42", resp.ID)
	}
}

func TestCreateTicket(t *testing.T) {
	var gotBody TicketCreate
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(TicketResponse{
			ID: 1, Title: gotBody.Title, Type: gotBody.Type, Status: "open", Priority: gotBody.Priority,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	desc := "detailed description"
	resp, err := c.CreateTicket("proj", TicketCreate{
		Title:       "New ticket",
		Description: &desc,
		Type:        "task",
		Priority:    "high",
		Tags:        []string{"backend"},
	})
	if err != nil {
		t.Fatalf("CreateTicket() error: %v", err)
	}
	if gotBody.Title != "New ticket" {
		t.Errorf("request title = %q, want %q", gotBody.Title, "New ticket")
	}
	if resp.ID != 1 {
		t.Errorf("ID = %d, want 1", resp.ID)
	}
}

func TestUpdateTicket(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/projects/proj/tickets/1" {
			t.Errorf("path = %s, want /projects/proj/tickets/1", r.URL.Path)
		}
		json.NewEncoder(w).Encode(TicketResponse{
			ID: 1, Title: "Updated", Type: "task", Status: "open", Priority: "low",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	title := "Updated"
	resp, err := c.UpdateTicket("proj", 1, TicketUpdate{Title: &title})
	if err != nil {
		t.Fatalf("UpdateTicket() error: %v", err)
	}
	if resp.Title != "Updated" {
		t.Errorf("Title = %q, want %q", resp.Title, "Updated")
	}
}

func TestTransitionTicket(t *testing.T) {
	var gotBody TransitionRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/projects/proj/tickets/1/move" {
			t.Errorf("path = %s, want /projects/proj/tickets/1/move", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(TicketResponse{
			ID: 1, Title: "Ticket", Type: "task", Status: gotBody.Status, Priority: "medium",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.TransitionTicket("proj", 1, "in_progress")
	if err != nil {
		t.Fatalf("TransitionTicket() error: %v", err)
	}
	if gotBody.Status != "in_progress" {
		t.Errorf("request status = %q, want %q", gotBody.Status, "in_progress")
	}
	if resp.Status != "in_progress" {
		t.Errorf("response status = %q, want %q", resp.Status, "in_progress")
	}
}

func TestSearchTickets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/tickets/search" {
			t.Errorf("path = %s, want /projects/proj/tickets/search", r.URL.Path)
		}
		if r.URL.Query().Get("query") != "auth" {
			t.Errorf("query = %q, want auth", r.URL.Query().Get("query"))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"tickets": []TicketResponse{{ID: 1, Title: "Auth bug", Type: "bug", Status: "open", Priority: "high"}},
			"total":   1,
			"query":   "auth",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.SearchTickets("proj", "auth", "", "", 0)
	if err != nil {
		t.Fatalf("SearchTickets() error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if resp.Query != "auth" {
		t.Errorf("Query = %q, want %q", resp.Query, "auth")
	}
}

func TestGetBoard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/tickets/board" {
			t.Errorf("path = %s, want /projects/proj/tickets/board", r.URL.Path)
		}
		if r.URL.Query().Get("view") != "summary" {
			t.Errorf("view = %q, want summary", r.URL.Query().Get("view"))
		}
		json.NewEncoder(w).Encode(BoardView{
			Columns: []BoardColumn{
				{Status: "open", Count: 3},
				{Status: "in_progress", Count: 1},
			},
			Total: 4,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetBoard("proj", "summary", "", "")
	if err != nil {
		t.Fatalf("GetBoard() error: %v", err)
	}
	if resp.Total != 4 {
		t.Errorf("Total = %d, want 4", resp.Total)
	}
	if len(resp.Columns) != 2 {
		t.Errorf("len(Columns) = %d, want 2", len(resp.Columns))
	}
}

func TestAddLink(t *testing.T) {
	var gotBody LinkCreate
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/projects/proj/tickets/1/links" {
			t.Errorf("path = %s, want /projects/proj/tickets/1/links", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(TicketLinkResp{
			ID: 10, SourceID: 1, TargetID: gotBody.TargetID, LinkType: gotBody.LinkType,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.AddLink("proj", 1, LinkCreate{TargetID: 2, LinkType: "blocks"})
	if err != nil {
		t.Fatalf("AddLink() error: %v", err)
	}
	if gotBody.TargetID != 2 {
		t.Errorf("request TargetID = %d, want 2", gotBody.TargetID)
	}
	if resp.LinkType != "blocks" {
		t.Errorf("LinkType = %q, want %q", resp.LinkType, "blocks")
	}
}

func TestListLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/tickets/1/links" {
			t.Errorf("path = %s, want /projects/proj/tickets/1/links", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]TicketLinkResp{
			{ID: 10, SourceID: 1, TargetID: 2, LinkType: "blocks"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListLinks("proj", 1)
	if err != nil {
		t.Fatalf("ListLinks() error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("len = %d, want 1", len(resp))
	}
	if resp[0].LinkType != "blocks" {
		t.Errorf("LinkType = %q, want %q", resp[0].LinkType, "blocks")
	}
}

// STOMPY-2527: _get_links_for_ticket's UNION ALL (stompy-ticketing
// service.py) always selects tl.* verbatim. Its second branch matches
// tl.target_id = ticket_id (viewing from the target's side) but the row's
// own target_id column is still the queried ticket's own id, not the far
// end — target_title/target_status correctly join to the source ticket,
// so only the id field is stale. ListLinks must normalize so TARGET ID
// names the OTHER ticket regardless of which side you queried from.
func TestListLinks_ReverseDirectionNormalizesTargetID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/tickets/2/links" {
			t.Errorf("path = %s, want /projects/proj/tickets/2/links", r.URL.Path)
		}
		// Link was created as source=1 -[related]-> target=2. Queried from
		// ticket 2 (the target), the server's raw row still carries
		// target_id=2 (itself) even though target_title/target_status
		// resolve to ticket 1, the real other end.
		json.NewEncoder(w).Encode([]TicketLinkResp{
			{ID: 1, SourceID: 1, TargetID: 2, LinkType: "related",
				TargetTitle: "Login fails with special characters in password", TargetStatus: "resolved"},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListLinks("proj", 2)
	if err != nil {
		t.Fatalf("ListLinks() error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("len(resp) = %d, want 1", len(resp))
	}
	if resp[0].TargetID != 1 {
		t.Errorf("TargetID = %d, want 1 (the other ticket, not the queried one)", resp[0].TargetID)
	}
	if resp[0].SourceID != 2 {
		t.Errorf("SourceID = %d, want 2 (the queried ticket)", resp[0].SourceID)
	}
	if resp[0].TargetTitle != "Login fails with special characters in password" {
		t.Errorf("TargetTitle = %q, unexpectedly changed", resp[0].TargetTitle)
	}
	if resp[0].TargetStatus != "resolved" {
		t.Errorf("TargetStatus = %q, unexpectedly changed", resp[0].TargetStatus)
	}
}

func TestRemoveLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/projects/proj/tickets/1/links/10" {
			t.Errorf("path = %s, want /projects/proj/tickets/1/links/10", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	err := c.RemoveLink("proj", 1, 10)
	if err != nil {
		t.Fatalf("RemoveLink() error: %v", err)
	}
}

func TestListTickets_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"message": "forbidden"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	_, err := c.ListTickets("proj", "", "", "", 0, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 403 {
		t.Errorf("StatusCode = %d, want 403", apiErr.StatusCode)
	}
}
