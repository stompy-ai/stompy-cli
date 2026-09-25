package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// STOMPY-2232 item 2: `stompy whoami` reads GET /billing/usage.
func TestClient_GetUsage_Measured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/billing/usage" {
			t.Errorf("wrong door: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"tier":"pro","units_used":241,"cap":300,"window":"calendar_month","period_start":"2026-09-01T00:00:00+00:00","period_end":"2026-10-01T00:00:00+00:00"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	usage, err := c.GetUsage()
	if err != nil {
		t.Fatalf("GetUsage() error: %v", err)
	}
	if usage.Tier != "pro" || usage.UnitsUsed == nil || *usage.UnitsUsed != 241 || usage.Cap == nil || *usage.Cap != 300 {
		t.Fatalf("usage = %+v", usage)
	}
	if usage.UnavailableReason != nil {
		t.Fatalf("unavailable_reason = %v, want nil", usage.UnavailableReason)
	}
}

// The meter can't invent a zero: unavailable_reason set means UnitsUsed
// stays nil, never 0.
func TestClient_GetUsage_Unavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"tier":"pro","unavailable_reason":"owner_unavailable"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	usage, err := c.GetUsage()
	if err != nil {
		t.Fatalf("GetUsage() error: %v", err)
	}
	if usage.UnitsUsed != nil {
		t.Fatalf("UnitsUsed = %v, want nil (never a fabricated 0)", *usage.UnitsUsed)
	}
	if usage.UnavailableReason == nil || *usage.UnavailableReason != "owner_unavailable" {
		t.Fatalf("UnavailableReason = %v", usage.UnavailableReason)
	}
}
