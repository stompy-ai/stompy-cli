package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// STOMPY-2722: `stompy context lifecycle <topic> --state ...` is the CLI door
// of the context lifecycle (park / deliver / archive / re-activate). These
// tests drive the REAL command tree (rootCmd.Execute) against an httptest
// server, so a command that is not registered fails here by name instead of
// failing to compile.

type lifecycleCall struct {
	method string
	path   string
	body   map[string]any
}

// runLifecycle executes `stompy <args...>` against h and returns what the
// user would see plus the request the server received.
func runLifecycle(t *testing.T, h http.HandlerFunc, args ...string) (stdout, stderr string, call lifecycleCall, err error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("STOMPY_API_KEY", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		call.method, call.path = r.Method, r.URL.EscapedPath()
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &call.body)
		}
		h(w, r)
	}))
	defer srv.Close()

	// cobra keeps flag values (and Changed) between Execute calls in one process.
	resetFlags := func(fs *pflag.FlagSet) {
		fs.VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue); f.Changed = false })
	}
	resetFlags(rootCmd.PersistentFlags())
	if c, _, ferr := rootCmd.Find([]string{"context", "lifecycle"}); ferr == nil && c != nil {
		resetFlags(c.Flags())
	}

	// PersistentPreRunE closes this channel once per Execute; a second Execute
	// in one process would close it twice. Give each run its own and wait for
	// the (instant, version "dev") check goroutine to finish with it.
	updateAvailable = make(chan string, 1)
	full := append([]string{"--api-url", srv.URL, "--api-key", "tok"}, args...)
	stdout = captureStdout(t, func() {
		stderr = captureStderr(t, func() {
			rootCmd.SetArgs(full)
			err = rootCmd.Execute()
		})
	})
	for range updateAvailable {
	}
	return
}

func jsonReply(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestContextLifecycle_SendsTheRESTContract(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantPath string
		wantBody map[string]any
	}{
		{
			name:     "park with a review window and a note",
			args:     []string{"-p", "proj", "-o", "json", "context", "lifecycle", "auth_arch", "--state", "parked", "--review-after-days", "30", "--note", "waiting on infra"},
			wantPath: "/projects/proj/contexts/auth_arch/lifecycle",
			wantBody: map[string]any{"state": "parked", "review_after_days": float64(30), "note": "waiting on infra"},
		},
		{
			name:     "re-activate sends only the state",
			args:     []string{"-p", "proj", "-o", "json", "context", "lifecycle", "auth_arch", "--state", "active"},
			wantPath: "/projects/proj/contexts/auth_arch/lifecycle",
			wantBody: map[string]any{"state": "active"},
		},
		{
			name:     "an explicit 0 days is sent (review now), not dropped as unset",
			args:     []string{"-p", "proj", "-o", "json", "context", "lifecycle", "auth_arch", "--state", "parked", "--review-after-days", "0"},
			wantPath: "/projects/proj/contexts/auth_arch/lifecycle",
			wantBody: map[string]any{"state": "parked", "review_after_days": float64(0)},
		},
		{
			name:     "a deeplink needs no -p",
			args:     []string{"-o", "json", "context", "lifecycle", "other/auth_arch", "--state", "archived"},
			wantPath: "/projects/other/contexts/auth_arch/lifecycle",
			wantBody: map[string]any{"state": "archived"},
		},
		{
			name:     "delivered",
			args:     []string{"-p", "proj", "-o", "json", "context", "lifecycle", "release_rules", "--state", "delivered"},
			wantPath: "/projects/proj/contexts/release_rules/lifecycle",
			wantBody: map[string]any{"state": "delivered"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reply := jsonReply(200, `{"topic":"auth_arch","lifecycle":"parked","previous":"active","changed":true,"changelogged":"parked","review_after":"2026-12-09"}`)
			_, _, call, err := runLifecycle(t, reply, tc.args...)
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if call.method != http.MethodPost || call.path != tc.wantPath {
				t.Errorf("request = %s %s, want POST %s", call.method, call.path, tc.wantPath)
			}
			if len(call.body) != len(tc.wantBody) {
				t.Errorf("body = %v, want %v", call.body, tc.wantBody)
			}
			for k, v := range tc.wantBody {
				if call.body[k] != v {
					t.Errorf("body[%s] = %v, want %v (body %v)", k, call.body[k], v, call.body)
				}
			}
		})
	}
}

func TestContextLifecycle_JSONIsTheAPIsObject(t *testing.T) {
	reply := jsonReply(200, `{"topic":"auth_arch","lifecycle":"parked","previous":"active","changed":true,"changelogged":"parked","review_after":"2026-12-09"}`)
	out, _, _, err := runLifecycle(t, reply, "-p", "proj", "-o", "json", "context", "lifecycle", "auth_arch", "--state", "parked")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	var got map[string]any
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("-o json output is not JSON: %v: %q", jerr, out)
	}
	want := map[string]any{"topic": "auth_arch", "lifecycle": "parked", "previous": "active", "changed": true, "changelogged": "parked", "review_after": "2026-12-09"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("json[%s] = %v, want %v", k, got[k], v)
		}
	}
}

func TestContextLifecycle_HumanOutput(t *testing.T) {
	cases := []struct {
		name  string
		reply string
		args  []string
		want  []string
	}{
		{
			name:  "park names the topic, the transition and the review date",
			reply: `{"topic":"auth_arch","lifecycle":"parked","previous":"active","changed":true,"changelogged":"parked","review_after":"2026-12-09"}`,
			args:  []string{"--state", "parked", "--review-after-days", "60"},
			want:  []string{"auth_arch", "active", "parked", "2026-12-09"},
		},
		{
			name:  "re-activation shows the way back and no review date",
			reply: `{"topic":"auth_arch","lifecycle":"active","previous":"archived","changed":true,"changelogged":"re-activated"}`,
			args:  []string{"--state", "active"},
			want:  []string{"auth_arch", "archived", "active"},
		},
		{
			name:  "an idempotent re-transition says nothing changed",
			reply: `{"topic":"auth_arch","lifecycle":"archived","changed":false,"note":"already in that state"}`,
			args:  []string{"--state", "archived"},
			want:  []string{"auth_arch", "archived", "already in that state"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"-p", "proj", "context", "lifecycle", "auth_arch"}, tc.args...)
			out, _, _, err := runLifecycle(t, jsonReply(200, tc.reply), args...)
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output %q does not contain %q", out, w)
				}
			}
			if tc.name == "re-activation shows the way back and no review date" && strings.Contains(out, "review") {
				t.Errorf("an active context has no review clock: %q", out)
			}
		})
	}
}

func TestContextLifecycle_RefusalsPassThroughVerbatim(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   []string
	}{
		{"missing topic", 404, `{"detail":"No context 'nope' in project 'proj'","error_code":"CONTEXT_NOT_FOUND"}`, []string{"404", "CONTEXT_NOT_FOUND", "No context 'nope' in project 'proj'"}},
		{"viewer", 403, `{"detail":"Your role on this shared project cannot modify it"}`, []string{"403", "Your role on this shared project cannot modify it"}},
		{"bad bound", 422, `{"detail":"review_after_days must be between 0 and 3650","error_code":"INVALID_REVIEW_AFTER_DAYS"}`, []string{"422", "INVALID_REVIEW_AFTER_DAYS", "between 0 and 3650"}},
		{"unit cap", 402, `{"detail":"Your plan holds 100 contexts.","error_code":"UNIT_CAP_REACHED"}`, []string{"402", "UNIT_CAP_REACHED", "Your plan holds 100 contexts."}},
		{"unauthenticated", 401, `{"detail":"Not authenticated"}`, []string{"401", "Not authenticated"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, _, _, err := runLifecycle(t, jsonReply(tc.status, tc.body),
				"-p", "proj", "context", "lifecycle", "auth_arch", "--state", "active")
			if err == nil {
				t.Fatalf("a refusal must be an error, got output %q", out)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not carry %q", err.Error(), w)
				}
			}
			if strings.Contains(out, "✓") {
				t.Errorf("a refusal must not print a success line: %q", out)
			}
		})
	}
}

func TestContextLifecycle_ClientSideUsageErrorsNeverReachTheServer(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"state is required", []string{"-p", "proj", "context", "lifecycle", "auth_arch"}, "--state"},
		{"unknown state names the vocabulary", []string{"-p", "proj", "context", "lifecycle", "auth_arch", "--state", "paused"}, "active, parked, delivered, archived"},
		{"negative days", []string{"-p", "proj", "context", "lifecycle", "auth_arch", "--state", "parked", "--review-after-days", "-3"}, "--review-after-days"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hit := false
			_, _, _, err := runLifecycle(t, func(w http.ResponseWriter, r *http.Request) { hit = true }, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one naming %q", err, tc.want)
			}
			if hit {
				t.Error("a usage error must not cost a request")
			}
		})
	}
}

func TestContextLifecycle_IsRegisteredUnderContext(t *testing.T) {
	c, _, err := rootCmd.Find([]string{"context", "lifecycle"})
	if err != nil || c == nil || c.Name() != "lifecycle" {
		t.Fatalf("`stompy context lifecycle` is not registered (found %v, err %v)", c, err)
	}
	for _, flag := range []string{"state", "review-after-days", "note"} {
		if c.Flags().Lookup(flag) == nil {
			t.Errorf("missing --%s", flag)
		}
	}
}
