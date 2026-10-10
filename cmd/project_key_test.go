package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// STOMPY-2705: `stompy project key mint|info|revoke|request` and
// `stompy project create --shared`.

func withAPI(t *testing.T, h http.HandlerFunc) func() {
	srv := httptest.NewServer(h)
	old := apiClient
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	return func() { apiClient = old; srv.Close() }
}

func TestProjectKeyMint_PrintsSecretToStdoutAndWarnsOnStderr(t *testing.T) {
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/proj/api-key" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"secret":"spk_abc","label":"guardian","project":"proj","rotated":true}`))
	})()
	projectKeyMintCmd.Flags().Set("label", "guardian")

	var stdout, stderr string
	stderr = captureStderr(t, func() {
		stdout = captureStdout(t, func() {
			if err := projectKeyMintCmd.RunE(projectKeyMintCmd, []string{"proj"}); err != nil {
				t.Fatalf("RunE() error: %v", err)
			}
		})
	})
	if !strings.Contains(stdout, "spk_abc") {
		t.Errorf("secret missing from stdout: %q", stdout)
	}
	if strings.Contains(stderr, "spk_abc") {
		t.Errorf("secret leaked to stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "ROTATES") {
		t.Errorf("stderr must warn that mint ROTATES: %q", stderr)
	}
}

func TestProjectKeyMint_JSONReturnsSecret(t *testing.T) {
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"secret":"spk_abc","label":"default","project":"proj","rotated":false}`))
	})()

	var out string
	withOutputFormat("json", func() {
		out = captureStdout(t, func() {
			captureStderr(t, func() {
				if err := projectKeyMintCmd.RunE(projectKeyMintCmd, []string{"proj"}); err != nil {
					t.Fatalf("RunE() error: %v", err)
				}
			})
		})
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v: %q", err, out)
	}
	if got["secret"] != "spk_abc" {
		t.Errorf("json missing secret: %+v", got)
	}
}

func TestProjectKeyMint_RefusalPassesThrough(t *testing.T) {
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"detail":"API keys cannot manage API keys — sign in to manage this project's key."}`))
	})()
	var err error
	captureStderr(t, func() { err = projectKeyMintCmd.RunE(projectKeyMintCmd, []string{"proj"}) })
	if err == nil || !strings.Contains(err.Error(), "cannot manage API keys") {
		t.Fatalf("refusal must reach the user, got %v", err)
	}
}

func TestProjectKeyInfo_NeverShowsASecret(t *testing.T) {
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		w.Write([]byte(`{"exists":true,"label":"guardian","created_at":"2026-10-01T10:00:00Z","last_used_at":null}`))
	})()
	out := captureStdout(t, func() {
		if err := projectKeyInfoCmd.RunE(projectKeyInfoCmd, []string{"proj"}); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})
	if !strings.Contains(out, "guardian") || strings.Contains(strings.ToLower(out), "spk_") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestProjectKeyInfo_NoKey(t *testing.T) {
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"exists":false}`)) })()
	out := captureStdout(t, func() {
		if err := projectKeyInfoCmd.RunE(projectKeyInfoCmd, []string{"proj"}); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})
	if !strings.Contains(out, "No active API key") {
		t.Errorf("output = %q", out)
	}
}

func TestProjectKeyRevoke_DeletesAndConfirms(t *testing.T) {
	called := false
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		called = r.Method == http.MethodDelete && r.URL.Path == "/projects/proj/api-key"
		w.WriteHeader(http.StatusNoContent)
	})()
	out := captureStdout(t, func() {
		if err := projectKeyRevokeCmd.RunE(projectKeyRevokeCmd, []string{"proj"}); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})
	if !called || !strings.Contains(out, "revoked") {
		t.Errorf("called=%v out=%q", called, out)
	}
}

func TestProjectKeyRevoke_NoActiveKeyIsAnError(t *testing.T) {
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"detail":"No active API key for this project."}`))
	})()
	err := projectKeyRevokeCmd.RunE(projectKeyRevokeCmd, []string{"proj"})
	if err == nil || !strings.Contains(err.Error(), "No active API key") {
		t.Fatalf("got %v", err)
	}
}

func TestProjectKeyRequest_ReportsNotified(t *testing.T) {
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/proj/api-key/request" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"notified":true}`))
	})()
	out := captureStdout(t, func() {
		if err := projectKeyRequestCmd.RunE(projectKeyRequestCmd, []string{"proj"}); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})
	if !strings.Contains(out, "notified") {
		t.Errorf("output = %q", out)
	}
}

func TestProjectCreateShared_PostsToSharedProjects(t *testing.T) {
	var path string
	defer withAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"project_id":"team_proj","name":"team_proj","is_shared":true,"my_role":"owner","created_at":"2026-10-01T10:00:00Z","member_count":1}`))
	})()
	projectCreateCmd.Flags().Set("shared", "true")
	defer projectCreateCmd.Flags().Set("shared", "false")

	out := captureStdout(t, func() {
		if err := projectCreateCmd.RunE(projectCreateCmd, []string{"team_proj"}); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})
	if path != "/shared-projects" {
		t.Errorf("path = %q, want /shared-projects", path)
	}
	if !strings.Contains(out, "team_proj") || !strings.Contains(out, "owner") {
		t.Errorf("output = %q", out)
	}
}

func TestProjectCreateShared_RejectsDescription(t *testing.T) {
	projectCreateCmd.Flags().Set("shared", "true")
	projectCreateCmd.Flags().Set("description", "x")
	defer func() {
		projectCreateCmd.Flags().Set("shared", "false")
		projectCreateCmd.Flags().Set("description", "")
	}()
	if err := projectCreateCmd.RunE(projectCreateCmd, []string{"team_proj"}); err == nil {
		t.Fatal("expected an error: shared projects take no description")
	}
}
