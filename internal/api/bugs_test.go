package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// STOMPY-1462 cluster 2: `bug list` was calling
// /projects/{project}/bug-reports, a path that has never existed — the
// server serves /bug-reports with no project filter, and its envelope key
// is "reports" (the CLI previously expected "bug_reports").

func TestListBugReports_UsesCorrectPathAndEnvelopeKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bug-reports" {
			t.Errorf("path = %s, want /bug-reports", r.URL.Path)
		}
		// The server's envelope key is "reports" — decode this raw so the
		// test fails if ListBugReports ever reverts to expecting
		// "bug_reports".
		w.Write([]byte(`{"reports":[{"id":"bug-uuid-1","title":"t","status":"new","severity":"high","created_at":"2026-08-23T14:50:00"}],"total":1}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListBugReports("myproj", "", 0, 0)
	if err != nil {
		t.Fatalf("ListBugReports() error: %v", err)
	}
	if resp.Total != 1 || len(resp.Reports) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	// A string id must round-trip — the CLI previously typed this as int.
	if resp.Reports[0].ID != "bug-uuid-1" {
		t.Errorf("ID = %q, want %q", resp.Reports[0].ID, "bug-uuid-1")
	}
}

func TestGetBugReport_StringID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/bug-reports/bug-uuid-1" {
			t.Errorf("path = %s, want /bug-reports/bug-uuid-1", r.URL.Path)
		}
		json.NewEncoder(w).Encode(BugReportResponse{ID: "bug-uuid-1", Title: "t", Status: "new"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetBugReport("myproj", "bug-uuid-1")
	if err != nil {
		t.Fatalf("GetBugReport() error: %v", err)
	}
	if resp.ID != "bug-uuid-1" {
		t.Errorf("ID = %q, want %q", resp.ID, "bug-uuid-1")
	}
}
