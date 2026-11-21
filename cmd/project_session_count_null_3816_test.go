package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// STOMPY-3816: since dementia #1809 (STOMPY-3808) the backend answers
// session_count=null when the count could not be read. Decoding null into an
// int printed a confident 0 the server never said.

func sessionCountServer(count string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := `{"context_count":4,"session_count":` + count + `,"file_count":0}`
		if strings.HasSuffix(r.URL.Path, "/projects") {
			_, _ = w.Write([]byte(`{"projects":[{"name":"proj","schema_name":"s_1","created_at":"2026-11-02T10:00:00Z","role":"owner","stats":` + stats + `}],"total":1}`))
			return
		}
		_, _ = w.Write([]byte(`{"name":"proj","schema_name":"s_1","created_at":"2026-11-02T10:00:00Z","role":"owner","stats":` + stats + `}`))
	}
}

func TestProjectInfo_NullSessionCountReadsUnknown3816(t *testing.T) {
	out, _, err := runCLI(t, sessionCountServer("null"), "project", "info", "proj", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"Sessions": "unknown"`) {
		t.Errorf("null session_count must read unknown: %q", out)
	}
}

func TestProjectList_NullSessionCountReadsUnknown3816(t *testing.T) {
	out, _, err := runCLI(t, sessionCountServer("null"), "project", "list", "--stats")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unknown") {
		t.Errorf("null session_count must read unknown in the list: %q", out)
	}
}

func TestProjectInfo_ZeroSessionCountStillReadsZero3816(t *testing.T) {
	out, _, err := runCLI(t, sessionCountServer("0"), "project", "info", "proj", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"Sessions": "0"`) || strings.Contains(out, "unknown") {
		t.Errorf("a real 0 stays 0: %q", out)
	}
}

func TestProjectList_PositiveSessionCountUnchanged3816(t *testing.T) {
	out, _, err := runCLI(t, sessionCountServer("7"), "project", "list", "--stats")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "7") || strings.Contains(out, "unknown") {
		t.Errorf("a real count is printed as is: %q", out)
	}
}
