package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// STOMPY-3500: the backend answers null storage for a project shared with
// you (it is not yours to size). Decoding null into an int printed "0 B",
// a confident zero the server never said.
func TestProjectInfo_NullStorageReadsNotShown3500(t *testing.T) {
	shared := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"proj","schema_name":"s_1","created_at":"2026-11-02T10:00:00Z","role":"viewer","stats":{"context_count":4,"session_count":0,"file_count":0,"storage_bytes_db":null,"storage_bytes_s3":null}}`))
	}
	out, _, err := runCLI(t, shared, "project", "info", "proj", "--stats")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "0 B") {
		t.Errorf("null storage must not print as 0 B: %q", out)
	}
	if strings.Count(out, "not shown") != 2 {
		t.Errorf("want both storage rows to read \"not shown\": %q", out)
	}
}

func TestProjectInfo_ZeroStorageStillReadsZero3500(t *testing.T) {
	out, _, err := runCLI(t, projectDetail, "project", "info", "proj", "--stats")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "0 B") || strings.Contains(out, "not shown") {
		t.Errorf("a real 0 stays 0 B: %q", out)
	}
}

// -o json renders the same rows as the table (not the raw response), so a
// null reads "not shown" there too, and a real size beside it is unaffected.
func TestProjectInfo_MixedNullAndSizeInJSON3500(t *testing.T) {
	mixed := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"proj","schema_name":"s_1","created_at":"2026-11-02T10:00:00Z","role":"viewer","stats":{"context_count":4,"session_count":0,"file_count":0,"storage_bytes_db":null,"storage_bytes_s3":2048}}`))
	}
	out, _, err := runCLI(t, mixed, "project", "info", "proj", "--stats", "-o", "json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"DB Storage": "not shown"`) || !strings.Contains(out, `"S3 Storage": "2.0 KB"`) {
		t.Errorf("want DB not shown and S3 2.0 KB: %q", out)
	}
}
