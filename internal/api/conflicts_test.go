package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// STOMPY-1462 cluster 2: `conflict list`/`detect` were calling
// /projects/{project}/conflicts, a path that has never existed — the server
// serves /conflicts with project as a QUERY param. These pin the correct
// wire contract (path, query param, and response shape).

func TestListConflicts_UsesQueryParamNotPathSegment(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conflicts" {
			t.Errorf("path = %s, want /conflicts", r.URL.Path)
		}
		if got := r.URL.Query().Get("project_id"); got != "myproj" {
			t.Errorf("project_id = %q, want %q", got, "myproj")
		}
		json.NewEncoder(w).Encode(ConflictListResponse{
			Conflicts: []ConflictResponse{
				{
					ID:                "conflict-uuid-1",
					ItemA:             ConflictItem{ItemType: "context", ItemID: 1},
					ItemB:             ConflictItem{ItemType: "context", ItemID: 2},
					ContradictionType: "direct",
					Status:            "pending",
					Confidence:        0.92,
				},
			},
			Total:        1,
			PendingCount: 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListConflicts("myproj", "", 0, 0)
	if err != nil {
		t.Fatalf("ListConflicts() error: %v", err)
	}
	if resp.Total != 1 || len(resp.Conflicts) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	// A string UUID id must round-trip — the CLI previously typed this as
	// int and would have failed to decode it at all.
	if resp.Conflicts[0].ID != "conflict-uuid-1" {
		t.Errorf("ID = %q, want %q", resp.Conflicts[0].ID, "conflict-uuid-1")
	}
	if resp.Conflicts[0].ItemA.ItemID != 1 {
		t.Errorf("ItemA.ItemID = %d, want 1", resp.Conflicts[0].ItemA.ItemID)
	}
}

func TestGetConflict_StringID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conflicts/abc-123-uuid" {
			t.Errorf("path = %s, want /conflicts/abc-123-uuid", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ConflictResponse{
			ID:     "abc-123-uuid",
			Status: "pending",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetConflict("myproj", "abc-123-uuid")
	if err != nil {
		t.Fatalf("GetConflict() error: %v", err)
	}
	if resp.ID != "abc-123-uuid" {
		t.Errorf("ID = %q, want %q", resp.ID, "abc-123-uuid")
	}
}

func TestDetectConflicts_SynchronousScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conflicts/detect" {
			t.Errorf("path = %s, want /conflicts/detect", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ConflictDetectResponse{
			ConflictsFound: 2,
			Pending:        2,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, queued, err := c.DetectConflicts("myproj", ConflictDetectRequest{Scope: "recent"})
	if err != nil {
		t.Fatalf("DetectConflicts() error: %v", err)
	}
	if queued != nil {
		t.Fatalf("expected no queued response for scope=recent, got %+v", queued)
	}
	if resp == nil || resp.ConflictsFound != 2 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestDetectConflicts_ScopeAllIsQueued(t *testing.T) {
	// STOMPY-1389: the server queues scope=all as a background job (202)
	// instead of blocking for 60-150s+. The CLI must recognize 202 and
	// decode the queued shape, not the completed-scan shape.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(ConflictDetectQueuedResponse{
			Status:  "queued",
			JobID:   "job-123",
			Scope:   "all",
			Message: "Scan queued",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, queued, err := c.DetectConflicts("myproj", ConflictDetectRequest{Scope: "all"})
	if err != nil {
		t.Fatalf("DetectConflicts() error: %v", err)
	}
	if resp != nil {
		t.Fatalf("expected no completed response for a queued scan, got %+v", resp)
	}
	if queued == nil || queued.JobID != "job-123" {
		t.Fatalf("unexpected queued response: %+v", queued)
	}
}

func TestResolveConflict_StringIDAndQueryParam(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/conflicts/uuid-1/resolve" {
			t.Errorf("path = %s, want /conflicts/uuid-1/resolve", r.URL.Path)
		}
		if got := r.URL.Query().Get("project_id"); got != "myproj" {
			t.Errorf("project_id = %q, want %q", got, "myproj")
		}
		json.NewEncoder(w).Encode(ConflictResponse{ID: "uuid-1", Status: "user_resolved"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ResolveConflict("myproj", "uuid-1", ConflictResolveRequest{Resolution: "keep_a"})
	if err != nil {
		t.Fatalf("ResolveConflict() error: %v", err)
	}
	if resp.Status != "user_resolved" {
		t.Errorf("Status = %q, want %q", resp.Status, "user_resolved")
	}
}
