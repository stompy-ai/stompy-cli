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
					fmt.Fprint(w, `{"detail":{"code":"UNIT_CAP_REACHED","message":"unit cap reached"}}`)
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
				if err == nil || !strings.Contains(err.Error(), "UNIT_CAP_REACHED") || out != "" {
					t.Fatalf("lost refusal or false success: err=%v out=%q", err, out)
				}
			} else if err != nil || !strings.Contains(out, "#7 closed") || !strings.Contains(out, "done") {
				t.Fatalf("lost server result: err=%v out=%q", err, out)
			}
		})
	}
}
