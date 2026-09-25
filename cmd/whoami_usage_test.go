package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/config"
)

// STOMPY-2232 item 2: `stompy whoami` shows usage (units/cap/tier) from
// GET /billing/usage when the backend provides it.
func TestWhoami_ShowsUsageFromBillingUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/billing/usage" {
			t.Errorf("wrong door: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"tier":"pro","units_used":241,"cap":300,"window":"calendar_month","period_start":"2026-09-01T00:00:00+00:00","period_end":"2026-10-01T00:00:00+00:00"}`)
	}))
	defer srv.Close()

	if err := config.Load(); err != nil {
		t.Fatal(err)
	}
	oldAPIKey, oldAPIURL := flagAPIKey, flagAPIURL
	flagAPIKey, flagAPIURL = "test-key", srv.URL
	defer func() { flagAPIKey, flagAPIURL = oldAPIKey, oldAPIURL }()

	out := captureStdout(t, func() {
		if err := whoamiCmd.RunE(whoamiCmd, nil); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})

	if !strings.Contains(out, "pro") {
		t.Errorf("output missing tier: %q", out)
	}
	if !strings.Contains(out, "241") || !strings.Contains(out, "300") {
		t.Errorf("output missing units used/cap: %q", out)
	}
}

// The meter can't measure: whoami must never print a fabricated 0, and
// must still answer the auth-status question.
func TestWhoami_UsageUnavailable_NeverFabricatesZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tier":"pro","unavailable_reason":"owner_unavailable"}`)
	}))
	defer srv.Close()

	if err := config.Load(); err != nil {
		t.Fatal(err)
	}
	oldAPIKey, oldAPIURL := flagAPIKey, flagAPIURL
	flagAPIKey, flagAPIURL = "test-key", srv.URL
	defer func() { flagAPIKey, flagAPIURL = oldAPIKey, oldAPIURL }()

	out := captureStdout(t, func() {
		if err := whoamiCmd.RunE(whoamiCmd, nil); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})

	if !strings.Contains(out, "Authenticated") {
		t.Fatalf("lost the auth status over a usage fetch problem: %q", out)
	}
	if strings.Contains(out, "0 / ") {
		t.Errorf("fabricated a zero usage reading: %q", out)
	}
	if !strings.Contains(out, "unavailable") {
		t.Errorf("output should name the unavailable reason: %q", out)
	}
}

// A server too old to have /billing/usage (404) must not break whoami's
// auth-status answer — usage is best-effort.
func TestWhoami_UsageFetchError_StillReportsAuthStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"not found"}`)
	}))
	defer srv.Close()

	if err := config.Load(); err != nil {
		t.Fatal(err)
	}
	oldAPIKey, oldAPIURL := flagAPIKey, flagAPIURL
	flagAPIKey, flagAPIURL = "test-key", srv.URL
	defer func() { flagAPIKey, flagAPIURL = oldAPIKey, oldAPIURL }()

	out := captureStdout(t, func() {
		if err := whoamiCmd.RunE(whoamiCmd, nil); err != nil {
			t.Fatalf("RunE() error: %v", err)
		}
	})
	if !strings.Contains(out, "Authenticated") {
		t.Fatalf("whoami must still answer even when usage can't be fetched: %q", out)
	}
}
