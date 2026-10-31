package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stompy-ai/stompy-cli/internal/api"
)

// STOMPY-3247 item 3: the server's refusal names the REST opt-in ("confirm": true in the
// body); a CLI user's way out is the --confirm flag, so the error must say so.
func TestTicketUpdateRefusalNamesConfirmFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		fmt.Fprint(w, `{"detail":"update would replace the stored description (12 chars) with text that does not contain it, so nothing was written. Send the description extended, or \"confirm\": true in the body to replace it.","error_code":"DESCRIPTION_WOULD_REPLACE","stored_length":12}`)
	}))
	defer srv.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "mine"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	cmd := ticketUpdateCmd
	_ = cmd.Flags().Set("description", "new body")
	defer func() { cmd.Flags().Lookup("description").Changed = false }()
	var err error
	captureStdout(t, func() { err = cmd.RunE(cmd, []string{"7"}) })
	if err == nil {
		t.Fatal("refusal lost")
	}
	if !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("the error must name --confirm: %v", err)
	}
	if !strings.Contains(err.Error(), "DESCRIPTION_WOULD_REPLACE") {
		t.Fatalf("lost the code: %v", err)
	}
}
