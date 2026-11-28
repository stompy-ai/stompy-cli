package cmd

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// STOMPY-3168 item 6: `stompy project delete "" --confirm` sent DELETE /projects/
// and printed "API error 405: Method Not Allowed". A blank name never reaches
// the API: it is refused locally with the sentence REST answers for a blank name.
func TestProjectDelete_RefusesBlankNameLocally(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	defer srv.Close()

	oldClient := apiClient
	apiClient = api.NewClient(srv.URL, "tok", "dev", false)
	defer func() { apiClient = oldClient }()

	for _, confirm := range []string{"true", "false"} {
		if err := projectDeleteCmd.Flags().Set("confirm", confirm); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"", " ", "\t\n"} {
			err := projectDeleteCmd.RunE(projectDeleteCmd, []string{name})
			if err == nil || err.Error() != "That is not a usable project name." {
				t.Errorf("delete %q (confirm=%s): got %v, want REST's authored sentence", name, confirm, err)
			}
		}
	}
	projectDeleteCmd.Flags().Set("confirm", "false")

	if n := atomic.LoadInt32(&calls); n != 0 {
		t.Errorf("a blank name reached the API %d time(s); it must be refused locally", n)
	}
}
