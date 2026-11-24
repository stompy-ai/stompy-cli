package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stompy-ai/stompy-cli/internal/config"
)

type session3849Call struct {
	Method, Path, Authorization, Grant, Refresh, Client string
}

type session3849 struct {
	t      *testing.T
	env    config.Environment
	server *httptest.Server
	mu     sync.Mutex
	calls  []session3849Call
}

// Each invocation uses the registered Cobra tree, real config persistence and
// real HTTP. Nothing replaces resolveAuthToken, RefreshToken or whoami.RunE.
func newSession3849(t *testing.T, env config.Environment, response string) *session3849 {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("STOMPY_API_KEY", "")
	viper.Reset()
	t.Cleanup(viper.Reset)
	if err := config.Load(); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range session3849Environments() {
		if err := config.SaveTokens(candidate, "expired-"+string(candidate), "refresh-"+string(candidate), time.Now().Add(-time.Hour), "fixture@example.invalid", ""); err != nil {
			t.Fatal(err)
		}
		if got := config.GetAccessToken(candidate); got != "expired-"+string(candidate) {
			t.Fatalf("stored token read-back: %q", got)
		}
	}
	fixture := &session3849{t: t, env: env}
	fixture.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		fixture.mu.Lock()
		fixture.calls = append(fixture.calls, session3849Call{
			r.Method, r.URL.Path, r.Header.Get("Authorization"),
			r.FormValue("grant_type"), r.FormValue("refresh_token"), r.FormValue("client_id"),
		})
		fixture.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			switch response {
			case "success":
				fmt.Fprint(w, `{"access_token":"renewed-access","refresh_token":"renewed-refresh","expires_in":3600}`)
			case "bad_json":
				fmt.Fprint(w, `{"access_token":!}`)
			default:
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"error":"invalid_grant","error_description":"refresh token expired"}`)
			}
		case "/api/v1/projects":
			fmt.Fprint(w, `{"projects":[],"total":0}`)
		case "/api/v1/billing/usage":
			w.WriteHeader(http.StatusNotFound) // whoami usage is best-effort.
		default:
			t.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func session3849Environments() []config.Environment {
	return []config.Environment{config.EnvProduction, config.EnvStaging}
}

func viperReload3849(t *testing.T) {
	t.Helper()
	viper.Reset()
	if err := config.Load(); err != nil {
		t.Fatal(err)
	}
}

func (f *session3849) run(args ...string) (string, error) {
	f.t.Helper()
	// Cobra retains flag values and Changed; restore them after this invocation.
	for _, flags := range []*pflag.FlagSet{rootCmd.PersistentFlags(), projectListCmd.Flags(), whoamiCmd.Flags()} {
		flags.VisitAll(func(flag *pflag.Flag) {
			value, changed := flag.Value.String(), flag.Changed
			f.t.Cleanup(func() { _ = flag.Value.Set(value); flag.Changed = changed })
			_ = flag.Value.Set(flag.DefValue)
			flag.Changed = false
		})
	}
	version, updates, client, mcp := Version, updateAvailable, apiClient, mcpClient
	Version, updateAvailable = "dev", make(chan string, 1)
	f.t.Cleanup(func() {
		Version, updateAvailable, apiClient, mcpClient = version, updates, client, mcp
		rootCmd.SetArgs(nil)
	})
	arguments := []string{"--api-url", f.server.URL + "/api/v1", "-o", "json"}
	if f.env == config.EnvStaging {
		arguments = append(arguments, "--use-staging")
	}
	rootCmd.SetArgs(append(arguments, args...))
	var err error
	out := captureStdout(f.t, func() { err = rootCmd.Execute() })
	for range updateAvailable {
	}
	return out, err
}

func (f *session3849) readConfig() string {
	f.t.Helper()
	data, err := os.ReadFile(config.GetConfigPath())
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *session3849) requests() []session3849Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]session3849Call(nil), f.calls...)
}

func (f *session3849) whoamiStatus() string {
	f.t.Helper()
	out, err := f.run("whoami")
	if err != nil {
		f.t.Fatal(err)
	}
	var fields map[string]string
	if err := json.Unmarshal([]byte(out), &fields); err != nil {
		f.t.Fatalf("whoami JSON: %v: %q", err, out)
	}
	if fields["Environment"] != string(f.env) {
		f.t.Fatalf("wrong environment: %v", fields)
	}
	return fields["Status"]
}
