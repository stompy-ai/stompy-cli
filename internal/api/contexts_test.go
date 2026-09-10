package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListContexts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/projects/myproj/contexts" {
			t.Errorf("path = %s, want /projects/myproj/contexts", r.URL.Path)
		}
		if r.URL.Query().Get("priority") != "important" {
			t.Errorf("priority = %q, want important", r.URL.Query().Get("priority"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit = %q, want 10", r.URL.Query().Get("limit"))
		}
		json.NewEncoder(w).Encode(ContextListResponse{
			Contexts: []ContextResponse{
				{ID: 1, Topic: "arch", Version: "1.0", Priority: "important", Tags: []string{"dev"}},
			},
			Total: 1,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListContexts("myproj", "important", "", 10, 0)
	if err != nil {
		t.Fatalf("ListContexts() error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if resp.Contexts[0].Topic != "arch" {
		t.Errorf("Topic = %q, want %q", resp.Contexts[0].Topic, "arch")
	}
}

func TestGetContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/myproj/contexts/arch_decisions" {
			t.Errorf("path = %s, want /projects/myproj/contexts/arch_decisions", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ContextDetailResponse{
			ContextResponse: ContextResponse{
				ID: 1, Topic: "arch_decisions", Version: "1.0", Priority: "important",
			},
			Content: "Use microservices",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetContext("myproj", "arch_decisions", "")
	if err != nil {
		t.Fatalf("GetContext() error: %v", err)
	}
	if resp.Content != "Use microservices" {
		t.Errorf("Content = %q, want %q", resp.Content, "Use microservices")
	}
}

func TestGetContext_WithVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("version") != "2.0" {
			t.Errorf("version = %q, want 2.0", r.URL.Query().Get("version"))
		}
		json.NewEncoder(w).Encode(ContextDetailResponse{
			ContextResponse: ContextResponse{ID: 1, Topic: "t", Version: "2.0"},
			Content:         "v2 content",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.GetContext("proj", "t", "2.0")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resp.Version != "2.0" {
		t.Errorf("Version = %q, want 2.0", resp.Version)
	}
}

func TestLockContext(t *testing.T) {
	var gotBody ContextCreateRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/projects/myproj/contexts" {
			t.Errorf("path = %s, want /projects/myproj/contexts", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(ContextCreateResponse{
			ID: 7, Topic: gotBody.Topic, Version: "1.0", URL: "https://stompy.ai/dashboard/projects/myproj/contexts/new_ctx@1.0",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	req := ContextCreateRequest{Topic: "new_ctx", Content: "content here", Priority: "important"}
	resp, err := c.LockContext("myproj", req)
	if err != nil {
		t.Fatalf("LockContext() error: %v", err)
	}
	if gotBody.Topic != "new_ctx" {
		t.Errorf("request topic = %q, want %q", gotBody.Topic, "new_ctx")
	}
	if resp.ID != 7 {
		t.Errorf("ID = %d, want 7", resp.ID)
	}
	if resp.URL == "" {
		t.Error("expected URL to be populated")
	}
}

func TestUnlockContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/projects/myproj/contexts/old_ctx" {
			t.Errorf("path = %s, want /projects/myproj/contexts/old_ctx", r.URL.Path)
		}
		if r.URL.Query().Get("force") != "true" {
			t.Errorf("force = %q, want true", r.URL.Query().Get("force"))
		}
		json.NewEncoder(w).Encode(ContextDeleteResponse{
			Topic: "old_ctx", Archived: true, DeletedCount: 1, VersionsDeleted: []string{"1 version(s)"},
			URL: "https://stompy.ai/dashboard/projects/myproj/contexts/old_ctx",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.UnlockContext("myproj", "old_ctx", "", true, false)
	if err != nil {
		t.Fatalf("UnlockContext() error: %v", err)
	}
	if resp.DeletedCount != 1 {
		t.Errorf("DeletedCount = %d, want 1", resp.DeletedCount)
	}
	if !resp.Archived {
		t.Error("expected Archived = true")
	}
}

func TestUpdateContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		json.NewEncoder(w).Encode(ContextResponse{
			ID: 1, Topic: "ctx", Version: "2.0", Priority: "always_check",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.UpdateContext("proj", "ctx", ContextUpdateRequest{Priority: "always_check"})
	if err != nil {
		t.Fatalf("UpdateContext() error: %v", err)
	}
	if resp.Priority != "always_check" {
		t.Errorf("Priority = %q, want %q", resp.Priority, "always_check")
	}
}

func TestSearchContexts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/contexts" {
			t.Errorf("path = %s, want /projects/proj/contexts", r.URL.Path)
		}
		if r.URL.Query().Get("search") != "architecture" {
			t.Errorf("search = %q, want architecture", r.URL.Query().Get("search"))
		}
		json.NewEncoder(w).Encode(ContextListResponse{Total: 2})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.SearchContexts("proj", "architecture", 0)
	if err != nil {
		t.Fatalf("SearchContexts() error: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("Total = %d, want 2", resp.Total)
	}
}

func TestMoveContext(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/projects/proj/contexts/ctx/move" {
			t.Errorf("path = %s, want /projects/proj/contexts/ctx/move", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotBody)
		json.NewEncoder(w).Encode(ContextMoveResponse{
			Status: "moved", Topic: "ctx", TargetProject: "other",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.MoveContext("proj", "ctx", "other")
	if err != nil {
		t.Fatalf("MoveContext() error: %v", err)
	}
	if gotBody["target_project"] != "other" {
		t.Errorf("target_project = %q, want %q", gotBody["target_project"], "other")
	}
	if resp.TargetProject != "other" {
		t.Errorf("TargetProject = %q, want %q", resp.TargetProject, "other")
	}
}
