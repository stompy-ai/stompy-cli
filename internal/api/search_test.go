package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// STOMPY-1967 item 5: GET /projects/{name}/search requires `query=` — the
// route's `query` parameter is required and unrelated to `q`. Sending `q=`
// (the old behavior) hits the live route as a 422
// ({"detail":[{"type":"missing","loc":["query","query"]...}]}) because
// `query` never arrives. Verified against staging 2026-09-06:
// `?query=Redis` -> 200, `?q=Redis` -> 422.
func TestSearch_SendsQueryParam(t *testing.T) {
	var gotQuery string
	var sawQKey bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/search" {
			t.Errorf("path = %s, want /projects/proj/search", r.URL.Path)
		}
		gotQuery = r.URL.Query().Get("query")
		_, sawQKey = r.URL.Query()["q"]
		json.NewEncoder(w).Encode(SearchResponse{Total: 0, Query: "Redis"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	if _, err := c.Search("proj", "Redis", 0); err != nil {
		t.Fatalf("Search() error: %v", err)
	}

	if gotQuery != "Redis" {
		t.Errorf("request query param = %q, want %q (the route requires query=, not q=)", gotQuery, "Redis")
	}
	if sawQKey {
		t.Errorf("request sent a q= param; the route only understands query=")
	}
}
