package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func Test3709AliasNoticeAllRESTCommands(t *testing.T) {
	for _, path := range []string{"/projects/old", "/projects/old/contexts", "/projects/old/tickets/7"} {
		t.Run(path, func(t *testing.T) {
			body := `{"now_called":"@alice/current","renamed_from":"@alice/old","content":"old stays authored data"}`
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer srv.Close()
			var data []byte
			stderr := captureStderr(t, func() {
				var err error
				data, _, err = NewClient(srv.URL, "tok", "dev", false).Do(http.MethodGet, path, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
			})
			if string(data) != body {
				t.Fatalf("response data rewritten: %s", data)
			}
			if !strings.Contains(stderr, `now called "@alice/current"`) || strings.Count(stderr, "\n") != 1 {
				t.Fatalf("want exactly one qualified notice, got %q", stderr)
			}
		})
	}
}

func Test3709AliasNoticeIgnoresRefusalsAndAuthoredData(t *testing.T) {
	for _, body := range []string{
		`{"content":{"now_called":"forged"}}`,
		`{"now_called":"forged","success":false}`,
		`{"now_called":"forged","error":"denied"}`,
		`{"now_called":"forged","enrichment_warning":"PROJECT_DISPLAY_UNAVAILABLE"}`,
		`{"name":"current"}`, `plain authored now_called text`,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
		stderr := captureStderr(t, func() { _, _, _ = NewClient(srv.URL, "tok", "dev", false).Do(http.MethodGet, "/test", nil, nil) })
		srv.Close()
		if stderr != "" {
			t.Fatalf("not a successful root notice: %s -> %q", body, stderr)
		}
	}
}

func Test3709AliasNoticeQuotesTerminalText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"now_called":"current\nFORGED","renamed_from":"old"}`)
	}))
	defer srv.Close()
	stderr := captureStderr(t, func() { _, _, _ = NewClient(srv.URL, "tok", "dev", false).Do(http.MethodGet, "/test", nil, nil) })
	if strings.Count(stderr, "\n") != 1 || !strings.Contains(stderr, `current\nFORGED`) {
		t.Fatalf("untrusted multiline notice: %q", stderr)
	}
}
