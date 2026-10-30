package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// STOMPY-3247: since stompy-ticketing 0.8.22 (STOMPY-2394) a description that
// drops the stored text is refused 409 unless the body says "confirm": true.
// --confirm is the CLI's only way to replace on purpose; without it the body
// must not carry the opt-in.
func TestTicketUpdateConfirmOptsInToAReplace(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		t.Run(fmt.Sprintf("confirm=%t", confirm), func(t *testing.T) {
			var body map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &body)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":7,"title":"t"}`)
			}))
			defer srv.Close()
			oldClient, oldProject := apiClient, flagProject
			apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "mine"
			defer func() { apiClient, flagProject = oldClient, oldProject }()
			cmd := ticketUpdateCmd
			_ = cmd.Flags().Set("description", "new body")
			_ = cmd.Flags().Set("confirm", fmt.Sprint(confirm))
			defer func() {
				_ = cmd.Flags().Set("confirm", "false")
				cmd.Flags().Lookup("confirm").Changed = false
				cmd.Flags().Lookup("description").Changed = false
			}()
			captureStdout(t, func() {
				if err := cmd.RunE(cmd, []string{"7"}); err != nil {
					t.Fatalf("update failed: %v", err)
				}
			})
			got, present := body["confirm"]
			if confirm && got != true {
				t.Fatalf(`--confirm must send "confirm": true, body=%v`, body)
			}
			if !confirm && present {
				t.Fatalf("without --confirm the body must not carry confirm, body=%v", body)
			}
		})
	}
}
