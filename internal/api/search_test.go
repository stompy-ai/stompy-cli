package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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
		json.NewEncoder(w).Encode(SearchResponse{TotalFound: 0})
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

// STOMPY-1968: GET /projects/{name}/search answers
// {"results":[{"id","label","preview","metadata":"<JSON string>",
// "bm25_score"/"rrf_score"/...,"source","scope"}],"total_found":N} — not
// the {"results":[{"topic","type","score","priority"}],"total","query"}
// shape this used to decode, which left every hit's Topic/Type/Score/
// Priority blank and Total always 0.
func TestSearch_DecodesLiveShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[` +
			`{"id":1,"label":"redis_notes","preview":"We use Redis.",` +
			`"metadata":"{\"priority\":\"reference\",\"tags\":[\"cache\"]}",` +
			`"bm25_score":0.1,"rrf_score":0.0164,"source":"bm25","scope":"project"}` +
			`],"total_found":1,"search_mode":"bm25_only"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.Search("proj", "Redis", 0)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if resp.TotalFound != 1 {
		t.Errorf("TotalFound = %d, want 1", resp.TotalFound)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(resp.Results))
	}
	got := resp.Results[0]
	if got.Label != "redis_notes" {
		t.Errorf("Label = %q, want %q", got.Label, "redis_notes")
	}
	if meta := got.DecodedMetadata(); meta.Priority != "reference" || len(meta.Tags) != 1 || meta.Tags[0] != "cache" {
		t.Errorf("DecodedMetadata() = %+v, want priority=reference tags=[cache]", meta)
	}
	if got.RankScore() != 0.0164 {
		t.Errorf("RankScore() = %v, want the rrf_score 0.0164", got.RankScore())
	}
	if got.Source != "bm25" || got.Scope != "project" {
		t.Errorf("Source/Scope = %q/%q, want bm25/project", got.Source, got.Scope)
	}
}

// TestSearch_LiveFixture decodes a RECORDED live response (staging
// 2026-09-06, project dogfood_cli_1968, GET /projects/{name}/search).
func TestSearch_LiveFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/search_results.json")
	if err != nil {
		t.Fatal(err)
	}
	var resp SearchResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if resp.TotalFound != 1 || len(resp.Results) != 1 {
		t.Fatalf("resp = %+v, want 1 result", resp)
	}
	got := resp.Results[0]
	if got.ID != 1 || got.Label != "kafka_notes" {
		t.Errorf("Results[0] = %+v, want id=1 label=kafka_notes", got)
	}
	if meta := got.DecodedMetadata(); meta.Priority != "important" {
		t.Errorf("DecodedMetadata().Priority = %q, want important", meta.Priority)
	}
	if got.RankScore() == 0 {
		t.Error("expected a non-zero RankScore() from the live fixture's rrf_score")
	}
}
