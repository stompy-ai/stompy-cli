package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// STOMPY-2712: project metadata + relations client calls. Responses are
// returned as the API object (map), never re-modelled, so -o json can print
// exactly what the server sent.

func relServer(t *testing.T, reply string, got *struct {
	method, path, rawQuery, ctype string
	body                          map[string]any
}) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path, got.rawQuery, got.ctype = r.Method, r.URL.EscapedPath(), r.URL.RawQuery, r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		got.body = nil
		if len(b) > 0 {
			_ = json.Unmarshal(b, &got.body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "tok", "test", false)
}

type seen = struct {
	method, path, rawQuery, ctype string
	body                          map[string]any
}

func TestSetProjectMetadata_PatchesOnlyTheFieldsGiven(t *testing.T) {
	var got seen
	c := relServer(t, `{"project":"my proj","schema":"s","kg":{"origin":"agent","purpose":"p"},"extra":1}`, &got)
	purpose := "p"
	origin := "agent"
	resp, err := c.SetProjectMetadata("my proj", ProjectMetadataUpdate{Origin: &origin, Purpose: &purpose, Clear: []string{"intent"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPatch || got.path != "/projects/my%20proj" || got.ctype != "application/json" {
		t.Errorf("got %s %s ctype=%q", got.method, got.path, got.ctype)
	}
	if len(got.body) != 3 || got.body["origin"] != "agent" || got.body["purpose"] != "p" {
		t.Errorf("body = %v (omitted fields must be absent, not null)", got.body)
	}
	if _, ok := resp["extra"]; !ok {
		t.Errorf("response must be the API object unchanged, got %v", resp)
	}
}

func TestLinkProject_PostsEdge(t *testing.T) {
	var got seen
	c := relServer(t, `{"project":"a","edge":{"relation_type":"feeds"}}`, &got)
	note := "why"
	if _, err := c.LinkProject("a", ProjectEdgeCreate{Target: "b", RelationType: "feeds", Note: &note}); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodPost || got.path != "/projects/a/edges" {
		t.Errorf("got %s %s", got.method, got.path)
	}
	if got.body["target"] != "b" || got.body["relation_type"] != "feeds" || got.body["note"] != "why" {
		t.Errorf("body = %v", got.body)
	}
	if _, has := got.body["grounding"]; has {
		t.Errorf("unset grounding must be omitted: %v", got.body)
	}
}

func TestUnlinkProject_DeleteWithTargetQuery(t *testing.T) {
	var got seen
	c := relServer(t, `{"project":"a","target":"b","removed":2}`, &got)
	resp, err := c.UnlinkProject("a", "b", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodDelete || got.path != "/projects/a/edges" || got.rawQuery != "target=b" {
		t.Errorf("got %s %s?%s (empty relation_type must be omitted)", got.method, got.path, got.rawQuery)
	}
	if resp["removed"] != float64(2) {
		t.Errorf("resp = %v", resp)
	}
	if _, err := c.UnlinkProject("@alice/a", "b", "feeds"); err != nil {
		t.Fatal(err)
	}
	if got.rawQuery != "owner=alice&relation_type=feeds&target=b" && got.rawQuery != "owner=alice&target=b&relation_type=feeds" {
		t.Errorf("query = %q, want owner + target + relation_type in one query string", got.rawQuery)
	}
}

func TestListProjectEdges_Gets(t *testing.T) {
	var got seen
	c := relServer(t, `{"project":"a","edges":[]}`, &got)
	if _, err := c.ListProjectEdges("a"); err != nil {
		t.Fatal(err)
	}
	if got.method != http.MethodGet || got.path != "/projects/a/edges" {
		t.Errorf("got %s %s", got.method, got.path)
	}
}

func TestPatchIsNotRetried(t *testing.T) {
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	p := "x"
	_, _ = NewClient(srv.URL, "tok", "t", false).SetProjectMetadata("a", ProjectMetadataUpdate{Purpose: &p})
	if n != 1 {
		t.Errorf("PATCH issued %d times; a write is sent once", n)
	}
}
