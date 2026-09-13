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

func TestTicketLeaseCommandsUseOneCanonicalWrite(t *testing.T) {
	for _, action := range []string{"claim", "release", "claim-next"} {
		t.Run(action, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				suffix := "7/" + action
				if action == "claim-next" {
					suffix = action
				}
				if r.Method != http.MethodPost || r.URL.Path != "/projects/mine/tickets/"+suffix || r.URL.Query().Get("owner") != "alice" {
					t.Errorf("wrong door: %s %s", r.Method, r.URL)
				}
				var payload map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload["agent_label"] != "Astra" {
					t.Errorf("lost label: %#v", payload)
				}
				if _, ok := payload["claimed_by"]; ok {
					t.Error("caller cannot choose authenticated holder")
				}
				if action == "claim-next" && payload["assignee"] != "Astra" {
					t.Error("lost lane")
				}
				fmt.Fprint(w, `{"status":"claimed","ticket":{"id":7,"claimed_by":{"account_id":51,"agent_label":"Astra"},"claimed_until":1789304400}}`)
			}))
			defer srv.Close()
			oldClient, oldProject := apiClient, flagProject
			apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "alice/mine"
			defer func() { apiClient, flagProject = oldClient, oldProject }()
			command := newTicketLeaseCmd(action)
			flags := []string{"--agent-label", "Astra"}
			args := []string{"7"}
			if action == "claim-next" {
				flags = append(flags, "--assignee", "Astra")
				args = nil
			}
			if err := command.ParseFlags(flags); err != nil {
				t.Fatal(err)
			}
			var out string
			withOutputFormat("json", func() {
				out = captureStdout(t, func() {
					if err := command.RunE(command, args); err != nil {
						t.Error(err)
					}
				})
			})
			if calls != 1 || !strings.Contains(out, `"account_id": 51`) {
				t.Fatalf("calls=%d, output=%s", calls, out)
			}
		})
	}
}

func TestTicketLeaseRefusalNeverPrintsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		fmt.Fprint(w, `{"detail":{"error":"LEASE_HELD","message":"held by Astra","claimed_by":{"account_id":51,"agent_label":"Astra"}}}`)
	}))
	defer srv.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "mine"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	command := newTicketLeaseCmd("claim")
	var err error
	out := captureStdout(t, func() { err = command.RunE(command, []string{"7"}) })
	if err == nil || !strings.Contains(err.Error(), "LEASE_HELD") || out != "" {
		t.Fatalf("err=%v output=%q", err, out)
	}
}
