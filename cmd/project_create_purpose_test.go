package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// STOMPY-3370: POST /projects has no `description` field (the server answers
// 422 for one since STOMPY-3343). The project's text lives in KG `purpose`,
// set by PATCH /projects/{name} — the same call `project set-metadata` makes.

const createdProj = `{"name":"proj","schema_name":"s_1","created_at":"2026-11-02T10:00:00Z","role":"owner"}`

// runCreate is runCLI that also clears the create flags afterwards: runCLI
// resets flags BEFORE a run, but the --shared tests call RunE directly and
// would otherwise inherit this file's --purpose.
func runCreate(t *testing.T, h http.HandlerFunc, args ...string) (string, []recordedReq, error) {
	t.Cleanup(func() {
		for _, name := range []string{"purpose", "description"} {
			_ = projectCreateCmd.Flags().Set(name, "")
			if f := projectCreateCmd.Flags().Lookup(name); f != nil {
				f.Changed = false
			}
		}
	})
	return runCLI(t, h, args...)
}

func createThenPatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodPatch {
		_, _ = w.Write([]byte(`{"project":"proj","kg":{"purpose":"ship it"}}`))
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write([]byte(createdProj))
}

func assertCreateThenPurpose(t *testing.T, reqs []recordedReq) {
	t.Helper()
	if len(reqs) != 2 {
		t.Fatalf("want POST then PATCH, got %+v", reqs)
	}
	if reqs[0].method != http.MethodPost || reqs[0].path != "/projects" {
		t.Errorf("first request = %s %s", reqs[0].method, reqs[0].path)
	}
	var create map[string]any
	if err := json.Unmarshal([]byte(reqs[0].body), &create); err != nil {
		t.Fatalf("create body %q: %v", reqs[0].body, err)
	}
	if _, has := create["description"]; has {
		t.Errorf("create must not send description (server 422s it): %v", create)
	}
	if reqs[1].method != http.MethodPatch || reqs[1].path != "/projects/proj" {
		t.Errorf("second request = %s %s", reqs[1].method, reqs[1].path)
	}
	var patch map[string]any
	if err := json.Unmarshal([]byte(reqs[1].body), &patch); err != nil {
		t.Fatalf("patch body %q: %v", reqs[1].body, err)
	}
	if patch["purpose"] != "ship it" || len(patch) != 1 {
		t.Errorf("patch body = %v, want only purpose", patch)
	}
}

func TestProjectCreate_PurposeIsSetByPatchAfterCreate(t *testing.T) {
	out, reqs, err := runCreate(t, createThenPatch, "project", "create", "proj", "--purpose", "ship it")
	if err != nil {
		t.Fatal(err)
	}
	assertCreateThenPurpose(t, reqs)
	if !strings.Contains(out, "ship it") {
		t.Errorf("table must show the purpose read back: %q", out)
	}
}

func TestProjectCreate_DescriptionIsADeprecatedAliasForPurpose(t *testing.T) {
	_, reqs, err := runCreate(t, createThenPatch, "project", "create", "proj", "--description", "ship it")
	if err != nil {
		t.Fatal(err)
	}
	assertCreateThenPurpose(t, reqs)
}

func TestProjectCreate_NoPurposeMakesOneRequest(t *testing.T) {
	_, reqs, err := runCreate(t, createThenPatch, "project", "create", "proj")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != http.MethodPost {
		t.Fatalf("reqs = %+v", reqs)
	}
}

func TestProjectCreate_PurposeFailureSaysTheProjectExists(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"detail":"purpose too long"}`))
			return
		}
		createThenPatch(w, r)
	}
	_, reqs, err := runCreate(t, h, "project", "create", "proj", "--purpose", "ship it")
	if err == nil {
		t.Fatal("expected the PATCH failure to surface")
	}
	if !strings.Contains(err.Error(), "created") || !strings.Contains(err.Error(), "set-metadata") ||
		!strings.Contains(err.Error(), "purpose too long") {
		t.Errorf("error must say the project exists, how to retry, and why: %v", err)
	}
	if len(reqs) != 2 {
		t.Errorf("reqs = %+v", reqs)
	}
}
