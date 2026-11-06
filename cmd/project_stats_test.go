package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// STOMPY-3473: GET /projects and GET /projects/{name} read the query param
// `include_stats`; `--stats` sent `stats`, which both routes silently ignore.
// The detail route returns counts on every read, so `project info` shows them
// without the flag; `--stats` adds the storage sizing only.

const statsBody = `{"context_count":18,"session_count":3,"file_count":2,"storage_bytes_db":2048,"storage_bytes_s3":0}`

func projectDetail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"name":"proj","schema_name":"s_1","created_at":"2026-11-02T10:00:00Z","role":"owner","stats":` + statsBody + `}`))
}

func projectList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"projects":[{"name":"proj","schema_name":"s_1","created_at":"2026-11-02T10:00:00Z","role":"owner","stats":` + statsBody + `}],"total":1}`))
}

func TestProjectInfo_StatsFlagSendsIncludeStats(t *testing.T) {
	_, reqs, err := runCLI(t, projectDetail, "project", "info", "proj", "--stats")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].query.Get("include_stats") != "true" {
		t.Fatalf("want ?include_stats=true, got %+v", reqs)
	}
	if reqs[0].query.Has("stats") {
		t.Errorf("must not send the ignored ?stats param: %v", reqs[0].query)
	}
}

func TestProjectList_StatsFlagSendsIncludeStats(t *testing.T) {
	_, reqs, err := runCLI(t, projectList, "project", "list", "--stats")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].query.Get("include_stats") != "true" {
		t.Fatalf("want ?include_stats=true, got %+v", reqs)
	}
}

func TestProjectInfo_ShowsTheCountsItReceivesWithoutTheFlag(t *testing.T) {
	out, reqs, err := runCLI(t, projectDetail, "project", "info", "proj")
	if err != nil {
		t.Fatal(err)
	}
	if reqs[0].query.Has("include_stats") {
		t.Errorf("no flag, no storage sizing: %v", reqs[0].query)
	}
	for _, want := range []string{"Contexts", "18", "Sessions", "Files"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q: %q", want, out)
		}
	}
	if strings.Contains(out, "DB Storage") {
		t.Errorf("storage is the --stats half: %q", out)
	}
}
