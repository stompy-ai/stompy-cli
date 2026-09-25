package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

// STOMPY-2232: the REST `usage_warning` sibling carries only used/cap/pct/
// resets_at/upgrade_url (STOMPY-2201) — never a tier name or a composed
// sentence, unlike the STOMPY-1870 cap-refusal shape. Line() is the CLI's
// own rendering of those numbers.
func TestUsageWarning_Line_MonthlyWithUpgrade(t *testing.T) {
	w := &UsageWarning{
		Used: 241, Cap: 300, Pct: 80,
		ResetsAt:   strPtr("2026-10-01T00:00:00+00:00"),
		UpgradeURL: strPtr("https://www.stompy.ai/dashboard/settings"),
	}
	line := w.Line()
	if !strings.HasPrefix(line, "Usage: 241 of 300 units this month — resets 1 Oct.") {
		t.Fatalf("line = %q", line)
	}
	if !strings.Contains(line, "Upgrade: https://www.stompy.ai/dashboard/settings") {
		t.Fatalf("line missing upgrade url: %q", line)
	}
}

func TestUsageWarning_Line_LifetimeNamesArchive(t *testing.T) {
	w := &UsageWarning{Used: 480, Cap: 500, Pct: 96}
	line := w.Line()
	if !strings.HasPrefix(line, "Usage: 480 of 500 active units (lifetime allowance).") {
		t.Fatalf("line = %q", line)
	}
	if !strings.Contains(line, "Archive") {
		t.Fatalf("free line should name archive: %q", line)
	}
	if strings.Contains(line, "resets") || strings.Contains(line, "Upgrade") {
		t.Fatalf("lifetime line should not mention resets/upgrade: %q", line)
	}
}

func TestUsageWarning_Line_MonthlyWithNoUpgradeNamesContactUs(t *testing.T) {
	// Power: no next rung (STOMPY-2202 NEXT_TIER), so upgrade_url is null
	// even though the cap is windowed.
	w := &UsageWarning{Used: 1000, Cap: 1200, Pct: 83, ResetsAt: strPtr("2026-10-01T00:00:00+00:00")}
	line := w.Line()
	if !strings.Contains(line, "Contact us: support@stompy.ai") {
		t.Fatalf("line = %q", line)
	}
}

func TestFormatThousands(t *testing.T) {
	cases := map[int]string{0: "0", 5: "5", 300: "300", 1200: "1,200", 1000000: "1,000,000"}
	for n, want := range cases {
		if got := formatThousands(n); got != want {
			t.Errorf("formatThousands(%d) = %q, want %q", n, got, want)
		}
	}
}

// captureStderr redirects os.Stderr for the duration of fn.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// STOMPY-2232: a 2xx write whose body carries `usage_warning` prints ONE
// line on stderr; stdout (the bytes Do() hands back to the caller, which the
// command layer then decodes/re-encodes) is untouched — the sibling never
// rides into a decoded response struct because none of them declare the
// field.
func TestClient_Do_PrintsUsageWarningOnStderrOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":1,"topic":"t","usage_warning":{"used":241,"cap":300,"pct":80,"resets_at":"2026-10-01T00:00:00+00:00","upgrade_url":"https://www.stompy.ai/dashboard/settings"}}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	var data []byte
	var err error
	stderr := captureStderr(t, func() {
		data, _, err = c.Do(http.MethodPost, "/test", map[string]string{"k": "v"}, nil)
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if !strings.HasPrefix(stderr, "Usage: 241 of 300 units this month — resets 1 Oct.") {
		t.Fatalf("stderr = %q", stderr)
	}

	var decoded struct {
		ID    int    `json:"id"`
		Topic string `json:"topic"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("body no longer valid JSON: %v", err)
	}
	if decoded.ID != 1 || decoded.Topic != "t" {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestClient_Do_NoUsageWarning_NoStderr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, `{"id":1,"topic":"t"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	var err error
	stderr := captureStderr(t, func() {
		_, _, err = c.Do(http.MethodPost, "/test", map[string]string{"k": "v"}, nil)
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty below the threshold", stderr)
	}
}

func TestClient_Do_ErrorResponse_NoUsageWarningStderr(t *testing.T) {
	// A refused write (403) never carries the sibling — the write did not
	// commit — and printUsageWarning must never run on the error path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"detail":"Pro plan: 300 of 300 units used this month (UTC).","error_code":"UNIT_CAP_REACHED"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "tok", "dev", false)
	var err error
	stderr := captureStderr(t, func() {
		_, _, err = c.Do(http.MethodPost, "/test", map[string]string{"k": "v"}, nil)
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty on a refusal", stderr)
	}
}
