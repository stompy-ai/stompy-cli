package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

func TestTicketCloseUsesCanonicalClose(t *testing.T) {
	for _, refused := range []bool{false, true} {
		t.Run(fmt.Sprintf("refused=%t", refused), func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost || r.URL.Path != "/projects/mine/tickets/7/close" {
					t.Errorf("noncanonical close: %s %s", r.Method, r.URL)
					http.Error(w, "wrong door", 400)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if refused {
					w.WriteHeader(http.StatusForbidden)
					// STOMPY-2223: the STOMPY-1870 wire — `detail` is the plain
					// sentence, `error_code` a sibling, not nested inside it.
					fmt.Fprint(w, `{"detail":"Pro plan: 300 of 300 units used this month (UTC). Resets on 2026-10-01.","error_code":"UNIT_CAP_REACHED","current":300,"cap":300,"resets_at":"2026-10-01T00:00:00+00:00","upgrade_url":"https://www.stompy.ai/dashboard/settings"}`)
					return
				}
				fmt.Fprint(w, `{"id":7,"status":"done"}`)
			}))
			defer srv.Close()
			oldClient, oldProject := apiClient, flagProject
			apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "mine"
			defer func() { apiClient, flagProject = oldClient, oldProject }()
			var err error
			out := captureStdout(t, func() { err = ticketCloseCmd.RunE(ticketCloseCmd, []string{"7"}) })
			if calls != 1 {
				t.Errorf("expected one canonical request, got %d", calls)
			}
			if refused {
				if err == nil || out != "" {
					t.Fatalf("lost refusal or false success: err=%v out=%q", err, out)
				}
				if !strings.Contains(err.Error(), "Pro plan: 300 of 300 units used this month") {
					t.Fatalf("lost the remedy sentence: err=%v", err)
				}
				if !strings.Contains(err.Error(), "UNIT_CAP_REACHED") {
					t.Fatalf("lost the error_code: err=%v", err)
				}
			} else if err != nil || !strings.Contains(out, "#7 closed") || !strings.Contains(out, "done") {
				t.Fatalf("lost server result: err=%v out=%q", err, out)
			}
		})
	}
}
