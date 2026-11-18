package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test3709NameEndpointsOwnerAndAuthentication(t *testing.T) {
	for _, method := range []string{"GET", "POST", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer tok" || r.URL.Query().Get("owner") != "alice" {
					t.Errorf("lost authority scope: %s %v", r.Header.Get("Authorization"), r.URL.Query())
				}
				want := "/projects/current/aliases"
				if method == "POST" {
					want = "/projects/current/rename"
				}
				if method == "DELETE" {
					want = "/projects/current/aliases/old"
				}
				if r.Method != method || r.URL.Path != want {
					t.Errorf("wrong endpoint: %s %s", r.Method, r.URL.Path)
				}
				fmt.Fprint(w, `{"success":true,"name":"current","schema_name":"physical","aliases":["old"],"released_alias":"old"}`)
			}))
			defer srv.Close()
			client := NewClient(srv.URL, "tok", "dev", false)
			switch method {
			case "GET":
				r, e := client.GetProjectAliases("@alice/current")
				if e != nil || r.Name != "current" || len(r.Aliases) != 1 {
					t.Fatalf("%+v %v", r, e)
				}
			case "POST":
				r, e := client.RenameProject("@alice/current", "next")
				if e != nil || !r.Success {
					t.Fatalf("%+v %v", r, e)
				}
			case "DELETE":
				r, e := client.ReleaseProjectName("@alice/current", "old")
				if e != nil || r.ReleasedAlias != "old" {
					t.Fatalf("%+v %v", r, e)
				}
			}
			if calls != 1 {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}

func Test3709ReleaseTransportNeverRetries(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(503); fmt.Fprint(w, `{}`) }))
	defer srv.Close()
	_, err := NewClient(srv.URL, "tok", "dev", false).ReleaseProjectName("current", "old")
	if err == nil || calls != 1 {
		t.Fatalf("ambiguous release retried: calls=%d err=%v", calls, err)
	}
}

func Test3709RenameRequestPreservesNewName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if json.NewDecoder(r.Body).Decode(&body) != nil || body["new_name"] != "New name" {
			t.Error("normalization belongs to the server", body)
		}
		fmt.Fprint(w, `{"success":true,"name":"new_name","schema_name":"physical","metadata_sync":"pending"}`)
	}))
	defer srv.Close()
	r, err := NewClient(srv.URL, "tok", "dev", false).RenameProject("current", "New name")
	if err != nil || r.Name != "new_name" || r.MetadataSync != "pending" {
		t.Fatalf("%+v %v", r, err)
	}
}
