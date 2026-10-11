package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var fixedTime = time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)

func TestListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/projects" {
			t.Errorf("path = %s, want /projects", r.URL.Path)
		}
		if r.URL.Query().Get("stats") != "true" {
			t.Error("expected stats=true query param")
		}
		json.NewEncoder(w).Encode(ProjectListResponse{
			Projects: []ProjectResponse{
				{Name: "proj1", SchemaName: "stompy_proj1", CreatedAt: fixedTime, Role: "owner"},
			},
			Total: 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListProjects(true)
	if err != nil {
		t.Fatalf("ListProjects() error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if resp.Projects[0].Name != "proj1" {
		t.Errorf("Name = %q, want %q", resp.Projects[0].Name, "proj1")
	}
}

func TestListProjects_WithoutStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("stats") != "" {
			t.Error("stats param should not be set")
		}
		json.NewEncoder(w).Encode(ProjectListResponse{Total: 0})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	_, err := c.ListProjects(false)
	if err != nil {
		t.Fatalf("ListProjects() error: %v", err)
	}
}

func TestGetProject(t *testing.T) {
	desc := "test project"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj" {
			t.Errorf("path = %s, want /projects/myproj", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ProjectResponse{
			Name:        "myproj",
			SchemaName:  "stompy_myproj",
			CreatedAt:   fixedTime,
			Role:        "owner",
			Description: &desc,
			Stats: &ProjectStats{
				ContextCount: 5,
				SessionCount: 10,
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetProject("myproj", true)
	if err != nil {
		t.Fatalf("GetProject() error: %v", err)
	}
	if resp.Name != "myproj" {
		t.Errorf("Name = %q, want %q", resp.Name, "myproj")
	}
	if resp.Stats == nil || resp.Stats.ContextCount != 5 {
		t.Errorf("Stats.ContextCount = %v, want 5", resp.Stats)
	}
}

func TestCreateProject(t *testing.T) {
	var gotBody ProjectCreate
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/projects" {
			t.Errorf("path = %s, want /projects", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ProjectResponse{
			Name:       gotBody.Name,
			SchemaName: "stompy_newproj",
			CreatedAt:  fixedTime,
			Role:       "owner",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.CreateProject(ProjectCreate{Name: "newproj"})
	if err != nil {
		t.Fatalf("CreateProject() error: %v", err)
	}
	if gotBody.Name != "newproj" {
		t.Errorf("request name = %q, want %q", gotBody.Name, "newproj")
	}
	if resp.Name != "newproj" {
		t.Errorf("response name = %q, want %q", resp.Name, "newproj")
	}
}

func TestDeleteProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/projects/oldproj" {
			t.Errorf("path = %s, want /projects/oldproj", r.URL.Path)
		}
		if r.URL.Query().Get("confirm") != "true" {
			t.Errorf("confirm = %q, want true", r.URL.Query().Get("confirm"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	err := c.DeleteProject("oldproj")
	if err != nil {
		t.Fatalf("DeleteProject() error: %v", err)
	}
}

func TestGetProject_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"message": "project not found"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	_, err := c.GetProject("missing", false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
}

// STOMPY-2742: the canonical `@owner/name` and wire `owner/name` both become
// GET /projects/{name}?owner=owner; a bare name stays bare.
func TestGetProjectQualifiedAddressSendsOwnerQuery(t *testing.T) {
	for _, addr := range []string{"@alice/shared", "alice/shared"} {
		var gotPath, gotOwner string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath, gotOwner = r.URL.EscapedPath(), r.URL.Query().Get("owner")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"shared","schema_name":"s","created_at":"2026-01-01T00:00:00Z","role":"member"}`))
		}))
		c := NewClient(srv.URL, "tok", "test", false)
		if _, err := c.GetProject(addr, false); err != nil {
			t.Fatalf("%s: %v", addr, err)
		}
		srv.Close()
		if gotPath != "/projects/shared" || gotOwner != "alice" {
			t.Errorf("%s -> %s ?owner=%s", addr, gotPath, gotOwner)
		}
	}
}
