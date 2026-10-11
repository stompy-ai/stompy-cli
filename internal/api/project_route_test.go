package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// STOMPY-2742 oracle: every project-scoped route builder goes through
// projectPath, so a qualified address (`@owner/proj` or `owner/proj`) can
// never again reach the wire as /projects/owner%2Fproj/... without ?owner=.
func TestNoRouteBuilderBypassesProjectPath(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	bad := regexp.MustCompile(`"/projects/%s|url\.PathEscape\((project|name)\)`)
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "projects.go" {
			continue // projects.go owns the helper
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if bad.MatchString(line) {
				t.Errorf("%s:%d builds a project route by hand (use projectPath): %s", f, i+1, strings.TrimSpace(line))
			}
		}
	}
}

func TestEveryProjectRouteSendsOwnerForQualifiedAddress(t *testing.T) {
	up := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(up, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls := map[string]func(c *Client, p string){
		"ListContexts":        func(c *Client, p string) { _, _ = c.ListContexts(p, "", "", 0, 0) },
		"GetContext":          func(c *Client, p string) { _, _ = c.GetContext(p, "t", "") },
		"LockContext":         func(c *Client, p string) { _, _ = c.LockContext(p, ContextCreateRequest{Topic: "t"}) },
		"UnlockContext":       func(c *Client, p string) { _, _ = c.UnlockContext(p, "t", "", true, false) },
		"UpdateContext":       func(c *Client, p string) { _, _ = c.UpdateContext(p, "t", ContextUpdateRequest{}) },
		"MoveContext":         func(c *Client, p string) { _, _ = c.MoveContext(p, "t", "other") },
		"SetContextLifecycle": func(c *Client, p string) { _, _ = c.SetContextLifecycle(p, "t", ContextLifecycleRequest{}) },
		"Search":              func(c *Client, p string) { _, _ = c.Search(p, "q", 5) },
		"ListTickets":         func(c *Client, p string) { _, _ = c.ListTickets(p, "", "", "", 0, 0) },
		"GetTicket":           func(c *Client, p string) { _, _ = c.GetTicket(p, 1) },
		"CreateTicket":        func(c *Client, p string) { _, _ = c.CreateTicket(p, TicketCreate{}) },
		"UpdateTicket":        func(c *Client, p string) { _, _ = c.UpdateTicket(p, 1, TicketUpdate{}) },
		"TransitionTicket":    func(c *Client, p string) { _, _ = c.TransitionTicket(p, 1, "done") },
		"CloseTicket":         func(c *Client, p string) { _, _ = c.CloseTicket(p, 1) },
		"SearchTickets":       func(c *Client, p string) { _, _ = c.SearchTickets(p, "q", "", "", 5) },
		"GetBoard":            func(c *Client, p string) { _, _ = c.GetBoard(p, "", "", "") },
		"AddLink":             func(c *Client, p string) { _, _ = c.AddLink(p, 1, LinkCreate{}) },
		"ListLinks":           func(c *Client, p string) { _, _ = c.ListLinks(p, 1) },
		"RemoveLink":          func(c *Client, p string) { _ = c.RemoveLink(p, 1, 2) },
		"LeaseTicket":         func(c *Client, p string) { _, _ = c.LeaseTicket(p, "claim", 1, TicketLeaseRequest{}) },
		"ArchiveTicket":       func(c *Client, p string) { _, _ = c.ArchiveTicket(p, "archive", 1) },
		"BatchArchiveTickets": func(c *Client, p string) { _, _ = c.BatchArchiveTickets(p, []int{1}, true) },
		"PostBoard":           func(c *Client, p string) { _, _ = c.PostBoard(p, BoardPostRequest{}) },
		"ReadBoard":           func(c *Client, p string) { _, _ = c.ReadBoard(p, "", "", 0) },
		"MintProjectKey":      func(c *Client, p string) { _, _ = c.MintProjectKey(p, "l") },
		"GetProjectKey":       func(c *Client, p string) { _, _ = c.GetProjectKey(p) },
		"RevokeProjectKey":    func(c *Client, p string) { _ = c.RevokeProjectKey(p) },
		"RequestProjectKey":   func(c *Client, p string) { _, _ = c.RequestProjectKey(p) },
		"ListFiles":           func(c *Client, p string) { _, _ = c.ListFiles(p, "", 0, 0) },
		"GetFile":             func(c *Client, p string) { _, _ = c.GetFile(p, 1) },
		"UploadFile":          func(c *Client, p string) { _, _ = c.UploadFile(p, up, "lbl") },
		"DeleteFile":          func(c *Client, p string) { _ = c.DeleteFile(p, 1) },
		"SetProjectMetadata":  func(c *Client, p string) { _, _ = c.SetProjectMetadata(p, ProjectMetadataUpdate{}) },
		"LinkProject":         func(c *Client, p string) { _, _ = c.LinkProject(p, ProjectEdgeCreate{Target: "t"}) },
		"UnlinkProject":       func(c *Client, p string) { _, _ = c.UnlinkProject(p, "t", "") },
		"ListProjectEdges":    func(c *Client, p string) { _, _ = c.ListProjectEdges(p) },
		"GetProject":          func(c *Client, p string) { _, _ = c.GetProject(p, false) },
		"DeleteProject":       func(c *Client, p string) { _ = c.DeleteProject(p) },
	}
	for name, call := range calls {
		for _, addr := range []string{"@alice/shared", "alice/shared"} {
			t.Run(name+"/"+addr, func(t *testing.T) {
				var path, owner string
				n := 0
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					n++
					path, owner = r.URL.EscapedPath(), r.URL.Query().Get("owner")
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{}`))
				}))
				defer srv.Close()
				call(NewClient(srv.URL, "tok", "test", false), addr)
				if n == 0 {
					t.Fatal("no request issued")
				}
				if !strings.HasPrefix(path, "/projects/shared") || strings.Contains(path, "%2F") || strings.Contains(path, "alice") {
					t.Errorf("path = %s, want /projects/shared...", path)
				}
				if owner != "alice" {
					t.Errorf("owner = %q, want alice (path %s)", owner, path)
				}
			})
		}
	}
}

func TestBareProjectNameIsUnchanged(t *testing.T) {
	if got := projectPath("my proj", "/contexts/t"); got != "/projects/my%20proj/contexts/t" {
		t.Errorf("got %s", got)
	}
	if got := projectPath("plain", ""); got != "/projects/plain" {
		t.Errorf("got %s", got)
	}
}
