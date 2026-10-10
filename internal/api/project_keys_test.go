package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// STOMPY-2705: project API key management + team project create.

func TestMintProjectKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/my%20proj/api-key" && r.URL.EscapedPath() != "/projects/my%20proj/api-key" {
			t.Errorf("got %s %s", r.Method, r.URL.EscapedPath())
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["label"] != "guardian" {
			t.Errorf("label = %q", body["label"])
		}
		json.NewEncoder(w).Encode(ProjectKeyMintResponse{Secret: "spk_test", Label: "guardian", Project: "my proj", Rotated: true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.MintProjectKey("my proj", "guardian")
	if err != nil {
		t.Fatalf("MintProjectKey() error: %v", err)
	}
	if resp.Secret != "spk_test" || !resp.Rotated || resp.Label != "guardian" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestGetProjectKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/projects/p/api-key" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"exists":true,"label":"guardian","created_at":"2026-10-01T10:00:00Z","last_used_at":null}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetProjectKey("p")
	if err != nil {
		t.Fatalf("GetProjectKey() error: %v", err)
	}
	if !resp.Exists || resp.Label == nil || *resp.Label != "guardian" || resp.LastUsedAt != nil {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestRevokeProjectKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/projects/p/api-key" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	if err := c.RevokeProjectKey("p"); err != nil {
		t.Fatalf("RevokeProjectKey() error: %v", err)
	}
}

func TestRequestProjectKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/p/api-key/request" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"notified":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.RequestProjectKey("p")
	if err != nil {
		t.Fatalf("RequestProjectKey() error: %v", err)
	}
	if !resp.Notified {
		t.Error("Notified = false, want true")
	}
}

func TestProjectKey_RefusalPassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"detail":"API keys cannot manage API keys — sign in to manage this project's key."}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "spk_x", "dev", false)
	_, err := c.MintProjectKey("p", "default")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 403 {
		t.Fatalf("want 403 APIError, got %v", err)
	}
	if !strings.Contains(err.Error(), "cannot manage API keys") {
		t.Errorf("refusal text lost: %v", err)
	}
}

func TestCreateSharedProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/shared-projects" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "team_proj" || len(body) != 1 {
			t.Errorf("body = %v", body)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"project_id":"team_proj","name":"team_proj","is_shared":true,"my_role":"owner","created_at":"2026-10-01T10:00:00Z","member_count":1}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.CreateSharedProject("team_proj")
	if err != nil {
		t.Fatalf("CreateSharedProject() error: %v", err)
	}
	if resp.ProjectID != "team_proj" || resp.MyRole != "owner" || !resp.IsShared {
		t.Errorf("unexpected response: %+v", resp)
	}
}

// The mint response body holds the only plaintext copy of the secret; --verbose
// previews response bodies to stderr, which must never include it.
func TestMintProjectKey_VerboseNeverPrintsSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"secret":"spk_SHOULD_NOT_APPEAR","label":"default","project":"p","rotated":false}`))
	}))
	defer srv.Close()

	old := os.Stderr
	rd, wr, _ := os.Pipe()
	os.Stderr = wr
	c := NewClient(srv.URL, "tok", "dev", true)
	_, err := c.MintProjectKey("p", "default")
	wr.Close()
	os.Stderr = old
	out, _ := io.ReadAll(rd)
	if err != nil {
		t.Fatalf("MintProjectKey() error: %v", err)
	}
	if strings.Contains(string(out), "spk_SHOULD_NOT_APPEAR") {
		t.Errorf("verbose log leaked the secret: %s", out)
	}
}
