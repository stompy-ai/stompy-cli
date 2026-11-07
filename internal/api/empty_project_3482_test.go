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
	for _, p := range []string{projectPath("", "/tickets"), "/projects/?x=1"} {
		if _, _, err := c.Do(http.MethodGet, p, nil, nil); err == nil || !strings.Contains(err.Error(), "project name is empty") {
			t.Errorf("Do(%q) err = %v, want the empty-name refusal", p, err)
		}
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

// The guard reads the shape projectPath builds; pin it so the two cannot drift.
func TestProjectPathShapeTheGuardReads(t *testing.T) {
	for in, want := range map[string]string{
		"":       "/projects/",
		"alice/": "/projects/?owner=alice",
	} {
		if got := projectPath(in, ""); got != want {
			t.Errorf("projectPath(%q) = %q, want %q", in, got, want)
		}
		if !emptyProjectSegment(projectPath(in, "")) {
			t.Errorf("emptyProjectSegment(projectPath(%q)) = false", in)
		}
	}
}
