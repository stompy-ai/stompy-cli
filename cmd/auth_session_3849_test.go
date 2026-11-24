package cmd

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stompy-ai/stompy-cli/internal/config"
)

func TestSession3849_RefreshFailureNamesEnvironment(t *testing.T) {
	for _, env := range session3849Environments() {
		t.Run(string(env), func(t *testing.T) {
			f := newSession3849(t, env, "invalid_grant")
			before := f.readConfig()
			out, err := f.run("project", "list")
			if err == nil || out != "" {
				t.Fatalf("expired session authenticated: stdout=%q error=%v", out, err)
			}
			hint := "stompy login"
			if env == config.EnvStaging {
				hint += " --use-staging"
			}
			for _, text := range []string{"your " + string(env) + " session expired and could not be renewed (", "token refresh failed (status 400)", "); run " + hint} {
				if !strings.Contains(err.Error(), text) {
					t.Errorf("refresh failure lost %q: %v", text, err)
				}
			}
			calls := f.requests()
			if len(calls) != 1 || calls[0].Method != "POST" || calls[0].Path != "/oauth/token" || calls[0].Grant != "refresh_token" || calls[0].Refresh != "refresh-"+string(env) || calls[0].Client != "stompy-cli" {
				t.Fatalf("wrong refresh/no-auth request sequence: %+v", calls)
			}
			if f.readConfig() != before {
				t.Fatal("failed refresh changed stored credentials")
			}
		})
	}
}

func TestSession3849_RefreshFailurePreservesCause(t *testing.T) {
	for _, env := range session3849Environments() {
		for _, cause := range []string{"bad_json", "network"} {
			t.Run(string(env)+"/"+cause, func(t *testing.T) {
				f := newSession3849(t, env, cause)
				if cause == "network" {
					f.server.Close()
				}
				before := f.readConfig()
				_, err := f.run("project", "list")
				if err == nil {
					t.Fatal("failed refresh authenticated")
				}
				var syntax *json.SyntaxError
				var network *url.Error
				if cause == "bad_json" && !errors.As(err, &syntax) || cause == "network" && !errors.As(err, &network) {
					t.Errorf("original refresh cause lost: %T: %v", err, err)
				}
				if !strings.Contains(err.Error(), "your "+string(env)+" session expired and could not be renewed") {
					t.Errorf("environment missing: %v", err)
				}
				if f.readConfig() != before {
					t.Fatal("failed refresh changed stored credentials")
				}
			})
		}
	}
}

func TestSession3849_FallbackStillAuthenticates(t *testing.T) {
	for _, env := range session3849Environments() {
		for _, source := range []string{"flag", "environment", "config"} {
			t.Run(string(env)+"/"+source, func(t *testing.T) {
				f := newSession3849(t, env, "invalid_grant")
				args := []string{"project", "list"}
				switch source {
				case "flag":
					args = append(args, "--api-key", "fallback-key")
				case "environment":
					t.Setenv("STOMPY_API_KEY", "fallback-key")
				case "config":
					if err := config.SetValue("api_key", "fallback-key"); err != nil {
						t.Fatal(err)
					}
				}
				before := f.readConfig()
				_, err := f.run(args...)
				if err != nil {
					t.Fatalf("valid fallback rejected: %v", err)
				}
				calls := f.requests()
				want := 1
				if source == "config" {
					want = 2
				}
				if len(calls) != want || calls[want-1].Path != "/api/v1/projects" || calls[want-1].Authorization != "Bearer fallback-key" {
					t.Fatalf("fallback precedence/request: %+v", calls)
				}
				if source == "config" && (calls[0].Path != "/oauth/token" || calls[0].Refresh != "refresh-"+string(env)) {
					t.Fatalf("config fallback skipped selected refresh: %+v", calls)
				}
				if f.readConfig() != before {
					t.Fatal("fallback changed stored credentials")
				}
			})
		}
	}
}

func TestSession3849_RefreshSuccessSavesSelectedEnvironment(t *testing.T) {
	for _, env := range session3849Environments() {
		t.Run(string(env), func(t *testing.T) {
			f := newSession3849(t, env, "success")
			_, err := f.run("project", "list")
			if err != nil {
				t.Fatal(err)
			}
			calls := f.requests()
			if len(calls) != 2 || calls[0].Refresh != "refresh-"+string(env) || calls[1].Authorization != "Bearer renewed-access" {
				t.Fatalf("wrong refresh/use sequence: %+v", calls)
			}
			// Read the file through a fresh Viper instance, not cached SaveTokens values.
			viperReload3849(t)
			for _, candidate := range session3849Environments() {
				access, refresh := "expired-"+string(candidate), "refresh-"+string(candidate)
				if candidate == env {
					access, refresh = "renewed-access", "renewed-refresh"
					if !config.GetTokenExpiry(env).After(time.Now().Add(30 * time.Minute)) {
						t.Fatal("renewed expiry was not persisted")
					}
				}
				if config.GetAccessToken(candidate) != access || config.GetRefreshToken(candidate) != refresh {
					t.Fatalf("wrong persisted %s credentials", candidate)
				}
			}
		})
	}
}

func TestSession3849_ValidOAuthSkipsRefresh(t *testing.T) {
	for _, env := range session3849Environments() {
		t.Run(string(env), func(t *testing.T) {
			f := newSession3849(t, env, "invalid_grant")
			if err := config.SaveTokens(env, "valid-access", "", time.Now().Add(time.Hour), "", ""); err != nil {
				t.Fatal(err)
			}
			before := f.readConfig()
			_, err := f.run("project", "list")
			calls := f.requests()
			if err != nil || len(calls) != 1 || calls[0].Authorization != "Bearer valid-access" || calls[0].Path != "/api/v1/projects" {
				t.Fatalf("valid OAuth failed: %v, %+v", err, calls)
			}
			if f.readConfig() != before {
				t.Fatal("valid session changed stored credentials")
			}
		})
	}
}

func TestSession3849_NoSessionKeepsGenericError(t *testing.T) {
	for _, env := range session3849Environments() {
		t.Run(string(env), func(t *testing.T) {
			f := newSession3849(t, env, "invalid_grant")
			if err := config.ClearTokens(env); err != nil {
				t.Fatal(err)
			}
			before := f.readConfig()
			_, err := f.run("project", "list")
			if err == nil || err.Error() != "not authenticated. Run 'stompy login' or set STOMPY_API_KEY" || len(f.requests()) != 0 {
				t.Fatalf("missing session contract changed: %v, %+v", err, f.requests())
			}
			if f.readConfig() != before {
				t.Fatal("missing session changed stored credentials")
			}
		})
	}
}
