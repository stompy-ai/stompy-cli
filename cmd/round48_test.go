package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

// STOMPY-3125 (dogfood round 48; also 2927 item 1 and 3083 item 8).

// replyMCPText answers every MCP tools/call with one text item, the way the
// server renders a tool's answer (TOON text, not JSON) — refusals included.
func replyMCPText(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, _ := json.Marshal(map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":` + string(result) + `}`))
	}
}

const invalidProject = "success: false\nerror: INVALID_PROJECT\nmessage: \"Project 'gone' does not exist.\""

// A tool that answers a refusal as text exits non-zero, naming the code: a
// script must be able to tell a revoked project from a result.
func TestToolTextRefusalExitsNonZero(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "gone", "context", "batch", "a", "b"},
		{"-p", "gone", "context", "explore"},
		{"-p", "gone", "context", "dashboard"},
		{"project", "brief", "gone"},
	} {
		t.Run(strings.Join(args[len(args)-2:], " "), func(t *testing.T) {
			_, _, err := runCLI(t, replyMCPText(invalidProject), args...)
			if err == nil {
				t.Fatalf("a refusal must be an error (exit 1)")
			}
			if !strings.Contains(err.Error(), "INVALID_PROJECT") ||
				!strings.Contains(err.Error(), "does not exist") {
				t.Errorf("the error must name the code and the message, got %v", err)
			}
		})
	}
}

func TestControlToolTextResultStillPrintsAndExitsZero(t *testing.T) {
	out, _, err := runCLI(t, replyMCPText("topic: a\nversion: 1.0\ncontent: hello"),
		"-p", "proj", "context", "batch", "a")
	if err != nil {
		t.Fatalf("a text result is not an error: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Errorf("text result must print verbatim: %q", out)
	}
}

// The upload of bytes the project already holds says so and names the file.
func TestFileUploadDuplicateSaysNotStoredAgain(t *testing.T) {
	path := t.TempDir() + "/mk_cli.txt"
	writeFile(t, path, "same bytes")
	out, _, err := runCLI(t, replyJSON(201, `{"id":7,"title":"mk_mcp.txt","created_at":"2026-10-22T00:00:00Z",`+
		`"metadata":{"size_bytes":10},"processing_status":"complete","is_duplicate":true,`+
		`"message":"Identical to existing file 'mk_mcp.txt' (id 7); not stored again under 'mk_cli.txt'."}`),
		"-p", "proj", "file", "upload", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "not stored again") || !strings.Contains(out, "mk_mcp.txt") {
		t.Errorf("a duplicate must say it was not stored and name the file: %q", out)
	}
}

func TestFileUploadReplaceNamesTheReplacedFile(t *testing.T) {
	path := t.TempDir() + "/survey_01.txt"
	writeFile(t, path, "new bytes")
	out, _, err := runCLI(t, replyJSON(201, `{"id":16,"title":"survey_01.txt","created_at":"2026-10-22T00:00:00Z",`+
		`"metadata":{"size_bytes":9},"processing_status":"pending","is_duplicate":false,"replaced_file_id":15,`+
		`"message":"File uploaded successfully"}`),
		"-p", "proj", "file", "upload", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Replaced") || !strings.Contains(out, "15") {
		t.Errorf("a replace must name the file it replaced: %q", out)
	}
}

// `context links` reads GET /projects/{p}/contexts/{topic}/edges.
func TestContextLinksReadsTheEdgesRoute(t *testing.T) {
	out, reqs, err := runCLI(t, replyJSON(200, `{"topic":"seed","links":[{"topic":"dst","link_type":"relates_to",`+
		`"provenance":"authored","weight":1}],"cited_by":[{"topic":"src","link_type":"blocks","provenance":"negotiated","weight":1}],"truncated":false}`),
		"-p", "proj", "context", "links", "seed")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != http.MethodGet || reqs[0].path != "/projects/proj/contexts/seed/edges" {
		t.Fatalf("reqs = %+v", reqs)
	}
	for _, want := range []string{"dst", "relates_to", "src", "blocks", "cited_by"} {
		if !strings.Contains(out, want) {
			t.Errorf("output must show %q: %q", want, out)
		}
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
