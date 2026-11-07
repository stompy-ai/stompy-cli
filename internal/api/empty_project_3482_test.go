package api

// STOMPY-3482: `stompy project info ""` exited 0 and printed a blank project dated
// 0001-01-01. An empty name built "/projects/", the server answered with the LIST
// body, and ProjectResponse decoded it as one zero-valued project. Every project
// route goes through projectPath and Do, so Do refuses an empty project segment
// before any request leaves.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmptyProjectNameIsRefusedBeforeAnyRequest(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{"projects":[],"total":0}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "t", "0.2.0", false)

	for _, name := range []string{"", "alice/"} {
		if _, err := c.GetProject(name, false); err == nil || !strings.Contains(err.Error(), "project name is empty") {
			t.Errorf("GetProject(%q) err = %v, want the empty-name refusal", name, err)
		}
	}
	if _, _, err := c.Do(http.MethodGet, projectPath("", "/tickets"), nil, nil); err == nil {
		t.Errorf("Do(/projects//tickets) = nil error, want the empty-name refusal")
	}
	if hits != 0 {
		t.Errorf("%d request(s) reached the server; an empty name must never leave", hits)
	}
}

func TestTheListRouteAndRealNamesStillGo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"projects":[],"total":0}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "t", "0.2.0", false)
	for _, p := range []string{"/projects", projectPath("real", ""), projectPath("alice/real", "/tickets")} {
		if _, _, err := c.Do(http.MethodGet, p, nil, nil); err != nil {
			t.Errorf("Do(%q) err = %v, want nil", p, err)
		}
	}
}
