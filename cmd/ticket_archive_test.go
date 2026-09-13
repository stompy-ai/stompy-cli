package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

func TestRestoreCapRefusalDoesNotPrintSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		fmt.Fprint(w, `{"detail":{"code":"UNIT_CAP_REACHED","message":"Archive or park a ticket first"}}`)
	}))
	defer server.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(server.URL, "tok", "dev", false), "mine"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	command := newTicketArchiveCmd("unarchive")
	var err error
	output := captureStdout(t, func() { err = command.RunE(command, []string{"7"}) })
	if err == nil || !strings.Contains(err.Error(), "UNIT_CAP_REACHED") || output != "" {
		t.Fatalf("err=%v output=%q", err, output)
	}
}

func TestBatchArchivePreviewAndConfirmAreExplicit(t *testing.T) {
	for _, confirm := range []bool{false, true} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			if r.Method != "POST" || r.URL.Path != "/projects/mine/tickets/batch/archive" {
				t.Errorf("wrong door %s %s", r.Method, r.URL)
			}
			var payload struct {
				TicketIDs []int `json:"ticket_ids"`
				Confirm   bool  `json:"confirm"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if fmt.Sprint(payload.TicketIDs) != "[7 8]" || payload.Confirm != confirm {
				t.Errorf("wrong payload %#v", payload)
			}
			fmt.Fprintf(w, `{"action":"batch_archive","dry_run":%t,"succeeded":2,"failed":0}`, !confirm)
		}))
		oldClient, oldProject := apiClient, flagProject
		apiClient, flagProject = api.NewClient(server.URL, "tok", "dev", false), "mine"
		command := newTicketArchiveCmd("batch-archive")
		if confirm {
			if err := command.Flags().Set("confirm", "true"); err != nil {
				t.Fatal(err)
			}
		}
		withOutputFormat("json", func() {
			captureStdout(t, func() {
				if err := command.RunE(command, []string{"7,8"}); err != nil {
					t.Error(err)
				}
			})
		})
		apiClient, flagProject = oldClient, oldProject
		server.Close()
		if calls != 1 {
			t.Fatalf("expected one batch request, got %d", calls)
		}
	}
}

func TestTicketArchiveAndRestoreUseTargetedPost(t *testing.T) {
	for _, action := range []string{"archive", "unarchive"} {
		t.Run(action, func(t *testing.T) {
			command, _, err := ticketCmd.Find([]string{action})
			if err != nil || command == ticketCmd {
				t.Fatalf("missing targeted command %s", action)
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/projects/mine/tickets/7/"+action || r.URL.Query().Get("owner") != "alice" {
					t.Errorf("wrong door: %s %s", r.Method, r.URL)
				}
				fmt.Fprint(w, `{"status":"archived","ticket":{"id":7,"status":"parked"}}`)
			}))
			defer server.Close()
			oldClient, oldProject := apiClient, flagProject
			apiClient, flagProject = api.NewClient(server.URL, "tok", "dev", false), "alice/mine"
			defer func() { apiClient, flagProject = oldClient, oldProject }()
			withOutputFormat("json", func() {
				output := captureStdout(t, func() {
					if err := command.RunE(command, []string{"7"}); err != nil {
						t.Error(err)
					}
				})
				if calls != 1 || !strings.Contains(output, `"parked"`) {
					t.Fatalf("calls=%d output=%s", calls, output)
				}
			})
		})
	}
}
