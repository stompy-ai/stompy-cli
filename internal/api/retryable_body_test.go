package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// STOMPY-2504: a 502/503/504 used to be treated as bare-retryable — Do()
// hit `continue` before ever looking at the body, so a deliberate
// STOMPY-1870 refusal riding a 503 (CODE_INDEX_DISABLED,
// BILLING_NOT_CONFIGURED, ADMISSION_BUSY, ...) printed only
// "API error 503: Service Unavailable" and threw the real sentence away.

// A 503 carrying the 1870 shape surfaces immediately, even on a POST
// (0 retries) where the old code already discarded the body on its only
// attempt.
func TestClient_Do_503WithStructuredBody_SurfacesDetailImmediately(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"detail":        "Nothing was written; retry in 5 s.",
			"error_code":    "ADMISSION_BUSY",
			"retry_after_s": 5,
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	_, code, err := c.Do(http.MethodPost, "/test", map[string]string{"k": "v"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", code)
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Detail != "Nothing was written; retry in 5 s." {
		t.Errorf("Detail = %q", apiErr.Detail)
	}
	if apiErr.ErrorCode != "ADMISSION_BUSY" {
		t.Errorf("ErrorCode = %q", apiErr.ErrorCode)
	}
	if got := apiErr.Error(); got != "API error 503 [ADMISSION_BUSY]: Service Unavailable — Nothing was written; retry in 5 s." {
		t.Errorf("Error() = %q", got)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (a structured refusal is not worth retrying blindly)", attempts)
	}
}

// The same structured surfacing on an idempotent method (GET), which would
// otherwise retry — a deliberate refusal still short-circuits the retries.
func TestClient_Do_503WithStructuredBody_SurfacesOnIdempotentTooWithoutRetrying(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"detail":     "The code index is disabled for this project.",
			"error_code": "CODE_INDEX_DISABLED",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	_, _, err := c.Do(http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !containsAll(err.Error(), "CODE_INDEX_DISABLED", "The code index is disabled for this project.") {
		t.Errorf("Error() = %q", err.Error())
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

// A 503 with no structured body (a proxy's plain-text or HTML page) keeps
// today's generic message and keeps retrying exactly as before.
func TestClient_Do_503WithoutBody_KeepsGenericMessage(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("<html><body>503 Service Unavailable</body></html>"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	_, _, err := c.Do(http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "API error 503: Service Unavailable" {
		t.Errorf("Error() = %q, want the plain generic message", got)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3 (1 initial + 2 retries, unchanged)", attempts)
	}
}

// A structured 503 with no `detail` (only, say, error_code) is not the 1870
// shape and does not short-circuit retries; retries exhaust and the
// generic message is used, same as an empty body.
func TestClient_Do_503WithoutDetail_StillRetriesAndExhausts(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{"status": "overloaded"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	_, _, err := c.Do(http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "API error 503: Service Unavailable" {
		t.Errorf("Error() = %q", got)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

// Retry-After (a genuinely transient 503, no structured detail) overrides
// the default exponential backoff for the next attempt.
func TestClient_Do_503HonoursRetryAfterHeader(t *testing.T) {
	attempts := 0
	var gaps []time.Duration
	var last time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		now := time.Now()
		if !last.IsZero() {
			gaps = append(gaps, now.Sub(last))
		}
		last = now
		if attempts <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("overloaded"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	data, code, err := c.Do(http.MethodGet, "/test", nil, nil)
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if code != 200 || string(data) != `{"ok":true}` {
		t.Fatalf("code=%d data=%s", code, data)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	for i, gap := range gaps {
		if gap < 900*time.Millisecond {
			t.Errorf("gap[%d] = %s, want >= ~1s (Retry-After honoured, not the faster exponential default)", i, gap)
		}
	}
}

func TestParseRetryAfterHeader(t *testing.T) {
	cases := map[string]time.Duration{
		"":     0,
		"5":    5 * time.Second,
		"0":    0,
		"-1":   0,
		"soon": 0, // HTTP-date form or garbage: fall back to the default backoff
	}
	for in, want := range cases {
		if got := parseRetryAfterHeader(in); got != want {
			t.Errorf("parseRetryAfterHeader(%q) = %s, want %s", in, got, want)
		}
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
