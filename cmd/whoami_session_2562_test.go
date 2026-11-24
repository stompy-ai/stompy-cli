package cmd

import (
	"testing"
	"time"

	"github.com/stompy-ai/stompy-cli/internal/config"
)

func TestSession2562_WhoamiExpiredWithoutRefreshNeedsLogin(t *testing.T) {
	for _, env := range session3849Environments() {
		t.Run(string(env), func(t *testing.T) {
			f := newSession3849(t, env, "invalid_grant")
			if err := config.SaveTokens(env, "expired-access", "", time.Now().Add(-time.Hour), "", ""); err != nil {
				t.Fatal(err)
			}
			before := f.readConfig()
			want := "Expired; run stompy login"
			if env == config.EnvStaging {
				want += " --use-staging"
			}
			if got := f.whoamiStatus(); got != want {
				t.Errorf("empty refresh token cannot renew: status=%q, want %q", got, want)
			}
			calls := f.requests()
			if len(calls) != 1 || calls[0].Path != "/api/v1/billing/usage" {
				t.Fatalf("whoami attempted refresh or another request: %+v", calls)
			}
			if f.readConfig() != before {
				t.Fatal("whoami changed stored credentials")
			}
		})
	}
}

func TestSession2562_WhoamiExpiredWithRefreshIsConditional(t *testing.T) {
	for _, env := range session3849Environments() {
		t.Run(string(env), func(t *testing.T) {
			f := newSession3849(t, env, "invalid_grant")
			before := f.readConfig()
			if got := f.whoamiStatus(); got != "Expired; renews on next command" {
				t.Errorf("refresh-capable status=%q", got)
			}
			calls := f.requests()
			if len(calls) != 1 || calls[0].Path != "/api/v1/billing/usage" {
				t.Fatalf("whoami attempted refresh: %+v", calls)
			}
			if f.readConfig() != before {
				t.Fatal("whoami changed stored credentials")
			}
		})
	}
}

func TestSession2562_WhoamiValidStaysValid(t *testing.T) {
	for _, env := range session3849Environments() {
		t.Run(string(env), func(t *testing.T) {
			f := newSession3849(t, env, "invalid_grant")
			if err := config.SaveTokens(env, "valid-access", "", time.Now().Add(time.Hour), "", ""); err != nil {
				t.Fatal(err)
			}
			before := f.readConfig()
			if got := f.whoamiStatus(); got != "Valid" {
				t.Errorf("valid token status=%q", got)
			}
			if f.readConfig() != before {
				t.Fatal("whoami changed stored credentials")
			}
		})
	}
}
