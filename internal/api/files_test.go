package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// STOMPY-1967 item 7: GET /projects/{name}/files answers
// {"documents":[{"id","title","file_type","size_bytes",...}],"total",
// "limit","offset"} — not the {"files":[{"filename","size",...}]} shape this
// used to decode.
func TestListFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/proj/files" {
			t.Errorf("path = %s, want /projects/proj/files", r.URL.Path)
		}
		w.Write([]byte(`{"documents":[` +
			`{"id":1,"title":"notes.txt","file_type":"txt","size_bytes":25,"processing_status":"complete","uploaded_at":"2026-09-06T16:41:00Z"}` +
			`],"total":1,"limit":50,"offset":0}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.ListFiles("proj", "", 0, 0)
	if err != nil {
		t.Fatalf("ListFiles() error: %v", err)
	}
	if resp.Total != 1 {
		t.Errorf("Total = %d, want 1", resp.Total)
	}
	if len(resp.Documents) != 1 {
		t.Fatalf("len(Documents) = %d, want 1 (the `documents` key was not decoded)", len(resp.Documents))
	}
	got := resp.Documents[0]
	if got.Title != "notes.txt" || got.SizeBytes != 25 || got.ProcessingStatus != "complete" {
		t.Errorf("Documents[0] = %+v, want title=notes.txt size_bytes=25 processing_status=complete", got)
	}
}

// TestListFiles_LiveFixture decodes a RECORDED live response (staging
// 2026-09-06, project dogfood_cli_1967, GET /projects/{name}/files).
func TestListFiles_LiveFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/file_list.json")
	if err != nil {
		t.Fatal(err)
	}
	var resp FileListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if resp.Total != 1 || len(resp.Documents) != 1 {
		t.Fatalf("resp = %+v, want 1 document", resp)
	}
	got := resp.Documents[0]
	if got.ID != 1 || got.Title != "dogfood_notes.txt" || got.SizeBytes != 25 || got.ProcessingStatus != "complete" {
		t.Errorf("Documents[0] = %+v", got)
	}
	if got.UploadedAt.IsZero() {
		t.Error("expected UploadedAt to be populated")
	}
}

// TestUploadFile_LiveFixture decodes a RECORDED live response (staging
// 2026-09-06, POST /projects/{name}/files). Unlike the list shape, size and
// type are nested under `metadata`, and the timestamp key is `created_at`.
func TestUploadFile_LiveFixture(t *testing.T) {
	body, err := os.ReadFile("testdata/file_upload.json")
	if err != nil {
		t.Fatal(err)
	}
	var resp FileUploadResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if resp.ID != 1 || resp.Title != "dogfood_notes.txt" {
		t.Errorf("resp = %+v, want id=1 title=dogfood_notes.txt", resp)
	}
	if resp.Metadata.SizeBytes != 25 {
		t.Errorf("Metadata.SizeBytes = %d, want 25 (nested under metadata, not top-level)", resp.Metadata.SizeBytes)
	}
	if resp.ProcessingStatus != "complete" {
		t.Errorf("ProcessingStatus = %q, want complete", resp.ProcessingStatus)
	}
	if resp.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be populated")
	}
}

// TestUploadFile verifies the multipart upload path end-to-end against the
// nested-metadata response shape, and that `label` is sent as a query
// parameter (the route reads it via FastAPI Query(), not as a form field).
func TestUploadFile(t *testing.T) {
	var gotLabel string
	var sawLabelForm bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		gotLabel = r.URL.Query().Get("label")
		if err := r.ParseMultipartForm(1 << 20); err == nil {
			if _, ok := r.MultipartForm.Value["label"]; ok {
				sawLabelForm = true
			}
		}
		w.Write([]byte(`{"id":5,"title":"x.txt","created_at":"2026-09-06T00:00:00Z",` +
			`"metadata":{"file_type":"document","size_bytes":3},"processing_status":"complete","message":"ok"}`))
	}))
	defer srv.Close()

	tmp, err := os.CreateTemp(t.TempDir(), "upload-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString("abc"); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	resp, err := c.UploadFile("proj", tmp.Name(), "mylabel")
	if err != nil {
		t.Fatalf("UploadFile() error: %v", err)
	}
	if resp.ID != 5 || resp.Metadata.SizeBytes != 3 {
		t.Errorf("resp = %+v, want id=5 metadata.size_bytes=3", resp)
	}
	if gotLabel != "mylabel" {
		t.Errorf("label query param = %q, want %q", gotLabel, "mylabel")
	}
	if sawLabelForm {
		t.Error("label was also sent as a multipart form field; the route only reads it from the query string")
	}
}
