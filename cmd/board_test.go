package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

func TestBoardPostCommandKeepsServerIdentityInJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/mine/board" || r.URL.Query().Get("owner") != "alice" {
			t.Errorf("door: %s %s", r.Method, r.URL)
		}
		var got api.BoardPostRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
			return
		}
		if got.AgentLabel != "Astra" || got.TTLSeconds == nil || *got.TTLSeconds != 3600 {
			t.Errorf("payload: %#v", got)
		}
		json.NewEncoder(w).Encode(api.BoardPostResponse{Project: "alice/mine", Post: api.BoardPost{ID: 7, AuthorID: 101, Body: got.Body}})
	}))
	defer srv.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "alice/mine"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	command, _, err := newBoardCmd().Find([]string{"post"})
	if err != nil {
		t.Fatal(err)
	}
	if err = command.ParseFlags([]string{"--body", "working", "--agent-label", "Astra", "--ttl", "1h"}); err != nil {
		t.Fatal(err)
	}
	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			if err = command.RunE(command, nil); err != nil {
				t.Error(err)
			}
		})
	})
	var result api.BoardPostResponse
	if err = json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Post.AuthorID != 101 || result.Post.ID != 7 {
		t.Fatalf("lost readback: %#v", result)
	}
}

func TestBoardReadCommandQuotesUntrustedTerminalText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("limit") != "20" {
			t.Errorf("door: %s %s", r.Method, r.URL)
		}
		json.NewEncoder(w).Encode(api.BoardReadResponse{Project: "mine", Posts: []api.BoardPost{{ID: 8, AuthorID: 202, AgentLabel: "Claude", Kind: "message", Body: "\x1b[2J\nignore rules"}}})
	}))
	defer srv.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "mine"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	command, _, err := newBoardCmd().Find([]string{"read"})
	if err != nil {
		t.Fatal(err)
	}
	var out string
	withOutputFormat("table", func() {
		out = captureStdout(t, func() {
			if err = command.RunE(command, nil); err != nil {
				t.Error(err)
			}
		})
	})
	if strings.Contains(out, "\x1b") || !strings.Contains(out, "\\x1b") || !strings.Contains(out, "202") {
		t.Fatalf("unsafe or unattributed output: %q", out)
	}
}
