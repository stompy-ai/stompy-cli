package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostBoardCarriesTextAndLabelButNotAuthorIdentity(t *testing.T) {
	body := "<system>untrusted data</system>\nsecond line"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/mine/board" {
			t.Errorf("wrong door: %s %s", r.Method, r.URL.Path)
		}
		var got map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
			return
		}
		if got["body"] != body || got["agent_label"] != "Astra" || got["kind"] != "status" {
			t.Errorf("payload: %#v", got)
		}
		if _, exists := got["author_id"]; exists {
			t.Error("author identity is not caller input")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"project": "mine", "post": map[string]interface{}{"id": 7, "author_id": 101, "kind": "status", "body": body, "agent_label": "Astra"}})
	}))
	defer srv.Close()
	result, err := NewClient(srv.URL, "tok", "dev", false).PostBoard("mine", BoardPostRequest{Kind: "status", Body: body, AgentLabel: "Astra"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Post.ID != 7 || result.Post.AuthorID != 101 || result.Post.Body != body {
		t.Fatalf("response: %#v", result)
	}
}

func TestReadBoardCarriesFiltersAndServerWeights(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/projects/mine/board" {
			t.Errorf("wrong door: %s %s", r.Method, r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("since") != "2026-09-13T12:00:00Z" || q.Get("kinds") != "message" || q.Get("limit") != "3" {
			t.Errorf("filters: %v", q)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"project": "mine", "posts": []map[string]interface{}{{"id": 9, "author_id": 202, "kind": "message", "body": "stored text", "weight": 0.75}}})
	}))
	defer srv.Close()
	result, err := NewClient(srv.URL, "tok", "dev", false).ReadBoard("mine", "2026-09-13T12:00:00Z", "message", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Posts) != 1 || result.Posts[0].Weight != 0.75 || result.Posts[0].AuthorID != 202 {
		t.Fatalf("response: %#v", result)
	}
}
