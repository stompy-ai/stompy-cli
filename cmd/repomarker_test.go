package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const cmdMarker = `version: 1
environment: production
projects:
  - address: "@alice/shared"
    role: shared
    default: true
  - address: mine
    role: primary
`

func repoDir(t *testing.T, marker string) string {
	t.Helper()
	dir := t.TempDir()
	if marker != "" {
		if err := os.WriteFile(filepath.Join(dir, ".stompy.yaml"), []byte(marker), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	return dir
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	return -1
}

func projectHandler(missing, forbidden string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case missing != "" && strings.HasSuffix(r.URL.Path, "/"+missing):
			w.WriteHeader(404)
			_, _ = w.Write([]byte(`{"detail":"Project not found"}`))
		case forbidden != "" && strings.HasSuffix(r.URL.Path, "/"+forbidden):
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"detail":"no access"}`))
		default:
			_, _ = w.Write([]byte(`{"name":"x","schema_name":"s","created_at":"2026-01-01T00:00:00Z","role":"member","is_system":false}`))
		}
	}
}

func TestProjectCheckAllResolveExitsZeroAndSendsOwner(t *testing.T) {
	repoDir(t, cmdMarker)
	out, reqs, err := runCLI(t, projectHandler("", ""), "project", "check")
	if exitCode(err) != 0 || err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("want 2 requests, got %d", len(reqs))
	}
	if reqs[0].path != "/projects/shared" || reqs[0].query.Get("owner") != "alice" {
		t.Errorf("shared project must be asked via ?owner=: %s %v", reqs[0].path, reqs[0].query)
	}
	if reqs[1].path != "/projects/mine" || reqs[1].query.Get("owner") != "" {
		t.Errorf("personal project must be a bare name: %s %v", reqs[1].path, reqs[1].query)
	}
	if strings.Count(out, "OK") != 2 {
		t.Errorf("want one OK line per address:\n%s", out)
	}
}

func TestProjectCheckNotFoundAndUnauthorizedExitOne(t *testing.T) {
	repoDir(t, cmdMarker)
	out, _, err := runCLI(t, projectHandler("shared", "mine"), "project", "check")
	if exitCode(err) != 1 {
		t.Fatalf("exit code = %d (%v), want 1", exitCode(err), err)
	}
	if !strings.Contains(out, "NOT FOUND") || !strings.Contains(out, "UNAUTHORIZED") {
		t.Errorf("each address needs its verdict:\n%s", out)
	}
}

func TestProjectCheckMissingFileExitsTwo(t *testing.T) {
	repoDir(t, "")
	_, reqs, err := runCLI(t, projectHandler("", ""), "project", "check")
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d (%v), want 2", exitCode(err), err)
	}
	if len(reqs) != 0 {
		t.Error("no request may be made without a marker")
	}
	if !strings.Contains(err.Error(), "stompy init") {
		t.Errorf("teach the fix: %v", err)
	}
}

func TestProjectCheckInvalidFileExitsTwo(t *testing.T) {
	repoDir(t, "version: 9\n")
	_, _, err := runCLI(t, projectHandler("", ""), "project", "check")
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d (%v), want 2", exitCode(err), err)
	}
}

func TestProjectCheckJSONOutput(t *testing.T) {
	repoDir(t, cmdMarker)
	out, _, err := runCLI(t, projectHandler("mine", ""), "-o", "json", "project", "check")
	if exitCode(err) != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode(err))
	}
	var rows []map[string]any
	if jerr := json.Unmarshal([]byte(out), &rows); jerr != nil {
		t.Fatalf("not JSON: %v\n%s", jerr, out)
	}
	if len(rows) != 2 || rows[0]["status"] != "ok" || rows[1]["status"] != "not_found" || rows[1]["address"] != "mine" {
		t.Errorf("unexpected rows: %v", rows)
	}
}

func TestProjectCheckWalksUpFromNestedDir(t *testing.T) {
	dir := repoDir(t, cmdMarker)
	nested := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	_, reqs, err := runCLI(t, projectHandler("", ""), "project", "check")
	if err != nil || len(reqs) != 2 {
		t.Fatalf("err=%v reqs=%d", err, len(reqs))
	}
}

func TestInitWritesMarkerAndManagedBlocks(t *testing.T) {
	dir := repoDir(t, "")
	out, _, err := runCLI(t, projectHandler("", ""), "init", "--project", "@markus/stompy", "--role", "primary", "--purpose", "the product")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	if !strings.Contains(out, "visible") {
		t.Errorf("must warn that handles are visible in a public repo:\n%s", out)
	}
	for _, f := range []string{".stompy.yaml", "AGENTS.md", "CLAUDE.md"} {
		if _, serr := os.Stat(filepath.Join(dir, f)); serr != nil {
			t.Errorf("%s not written", f)
		}
	}
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	claude, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if !strings.Contains(string(agents), `project="markus/stompy"`) || string(agents) != string(claude) {
		t.Errorf("blocks missing or differ:\n%s", agents)
	}
}

func TestInitIsIdempotentAndPreservesOutsideText(t *testing.T) {
	dir := repoDir(t, "")
	pre := "# Rules\n\nKeep me.\n"
	if err := os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(pre), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := runCLI(t, projectHandler("", ""), "init", "--project", "mine"); err != nil {
		t.Fatal(err)
	}
	first := map[string]string{}
	for _, f := range []string{".stompy.yaml", "AGENTS.md", "CLAUDE.md"} {
		b, _ := os.ReadFile(filepath.Join(dir, f))
		first[f] = string(b)
	}
	if !strings.HasPrefix(first["CLAUDE.md"], pre) {
		t.Fatalf("outside text touched:\n%s", first["CLAUDE.md"])
	}
	// second run, flagless refresh AND identical flags: both no-ops
	for _, args := range [][]string{{"init"}, {"init", "--project", "mine"}} {
		if _, _, err := runCLI(t, projectHandler("", ""), args...); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		for f, want := range first {
			b, _ := os.ReadFile(filepath.Join(dir, f))
			if string(b) != want {
				t.Errorf("%v changed %s", args, f)
			}
		}
	}
}

func TestInitRefusesToOverwriteWithoutForce(t *testing.T) {
	dir := repoDir(t, cmdMarker)
	_, _, err := runCLI(t, projectHandler("", ""), "init", "--project", "other")
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("want a --force refusal, got %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, ".stompy.yaml"))
	if string(b) != cmdMarker {
		t.Error("file was modified despite refusal")
	}
	if _, _, err := runCLI(t, projectHandler("", ""), "init", "--project", "other", "--force"); err != nil {
		t.Fatalf("--force: %v", err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, ".stompy.yaml"))
	if !strings.Contains(string(b), "other") {
		t.Error("--force did not overwrite")
	}
}

func TestInitWithoutProjectOrFileTeaches(t *testing.T) {
	repoDir(t, "")
	_, _, err := runCLI(t, projectHandler("", ""), "init")
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("want a teaching error, got %v", err)
	}
}
