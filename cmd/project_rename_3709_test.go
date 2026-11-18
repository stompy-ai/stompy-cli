package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func renameHandler3709(mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "POST":
			if mode == "refused" {
				w.WriteHeader(403)
				fmt.Fprint(w, `{"error_code":"RENAME_FORBIDDEN","detail":"Owner only"}`)
				return
			}
			sync := "complete"
			if mode == "pending" {
				sync = "pending"
			}
			fmt.Fprintf(w, `{"success":true,"name":"current","schema_name":"physical_old","changed":true,"metadata_sync":%q}`, sync)
		case strings.HasSuffix(r.URL.Path, "/aliases"):
			fmt.Fprint(w, `{"name":"current","schema_name":"physical_old","aliases":["old"]}`)
		case r.URL.Path == "/projects/old":
			fmt.Fprint(w, `{"name":"old","schema_name":"physical_old","created_at":"2026-11-17T00:00:00Z","role":"owner"}`)
		default:
			physical := "physical_old"
			if mode == "recreated" {
				physical = "different"
			}
			fmt.Fprintf(w, `{"name":"current","schema_name":%q,"created_at":"2026-11-17T00:00:00Z","role":"owner"}`, physical)
		}
	}
}

func Test3709RenameQualifiedRoundTrip(t *testing.T) {
	out, reqs, err := runCLI(t, renameHandler3709("complete"), "-o", "json", "project", "rename", "@alice/old", "current")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /projects/old", "POST /projects/old/rename", "GET /projects/current", "GET /projects/current/aliases"}
	if len(reqs) != len(want) {
		t.Fatalf("want read/write/readbacks, got %+v", reqs)
	}
	for i, r := range reqs {
		if r.method+" "+r.path != want[i] || r.query.Get("owner") != "alice" {
			t.Errorf("request%d: %+v", i, r)
		}
	}
	if reqs[1].body != `{"new_name":"current"}` {
		t.Errorf("wrong rename: %s", reqs[1].body)
	}
	var result map[string]any
	if json.Unmarshal([]byte(out), &result) != nil || result["name"] != "current" || result["verified"] != true {
		t.Fatalf("not verified JSON: %s", out)
	}
}

func Test3709RenameRecreateRefusesVerifiedSuccess(t *testing.T) {
	out, reqs, err := runCLI(t, renameHandler3709("recreated"), "project", "rename", "old", "current")
	if err == nil || !strings.Contains(err.Error(), "committed") || strings.Contains(out, "verified") {
		t.Fatalf("must expose committed unverified outcome: %v %s", err, out)
	}
	posts := 0
	for _, r := range reqs {
		if r.method == "POST" {
			posts++
		}
	}
	if posts != 1 {
		t.Fatalf("never retry rename: %+v", reqs)
	}
}

func Test3709RenamePendingIsCommittedSuccess(t *testing.T) {
	out, reqs, err := runCLI(t, renameHandler3709("pending"), "-o", "json", "project", "rename", "old", "current")
	if err != nil || !strings.Contains(out, `"metadata_sync": "pending"`) {
		t.Fatalf("pending was treated as failure: %v %s", err, out)
	}
	posts := 0
	for _, r := range reqs {
		if r.method == "POST" {
			posts++
		}
	}
	if posts != 1 {
		t.Fatal(reqs)
	}
}

func Test3709RenameRefusalControl(t *testing.T) {
	out, reqs, err := runCLI(t, renameHandler3709("refused"), "project", "rename", "old", "current")
	if err == nil || !strings.Contains(err.Error(), "RENAME_FORBIDDEN") || out != "" {
		t.Fatalf("refusal hidden: %v %s", err, out)
	}
	if len(reqs) != 2 {
		t.Fatalf("readback or retry after refusal: %+v", reqs)
	}
}

func Test3709RenameArityNoNetwork(t *testing.T) {
	for _, args := range [][]string{{}, {"old"}, {"old", "new", "extra"}} {
		_, reqs, err := runCLI(t, renameHandler3709("complete"), append([]string{"project", "rename"}, args...)...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("arity %v: %v %+v", args, err, reqs)
		}
	}
}

func Test3709RenameEmptyInputNoNetwork(t *testing.T) {
	for _, args := range [][]string{{"", "current"}, {"old", "  "}} {
		_, reqs, err := runCLI(t, renameHandler3709("complete"), append([]string{"project", "rename"}, args...)...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("empty input sent: %v %+v", err, reqs)
		}
	}
}

func Test3709RenameAliasReadbackFailureIsVisible(t *testing.T) {
	for _, body := range []string{
		`{"name":"later","schema_name":"physical_old","aliases":["old"]}`,
		`{"name":"current","schema_name":"different","aliases":["old"]}`,
		`{"name":"current","schema_name":"physical_old"}`,
	} {
		out, reqs, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/aliases") {
				fmt.Fprint(w, body)
				return
			}
			renameHandler3709("complete")(w, r)
		}, "project", "rename", "old", "current")
		if err == nil || !strings.Contains(err.Error(), "committed") || out != "" {
			t.Fatalf("false success on alias evidence: %v %s", err, out)
		}
		posts := 0
		for _, r := range reqs {
			if r.method == "POST" {
				posts++
			}
		}
		if posts != 1 {
			t.Fatal(reqs)
		}
	}
}

func Test3709RenameLaterNameDoesNotBecomeSuccess(t *testing.T) {
	_, _, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/projects/current" {
			fmt.Fprint(w, `{"name":"later","schema_name":"physical_old","created_at":"2026-11-17T00:00:00Z","now_called":"later","renamed_from":"current"}`)
			return
		}
		renameHandler3709("complete")(w, r)
	}, "project", "rename", "old", "current")
	if err == nil || !strings.Contains(err.Error(), "committed") {
		t.Fatalf("later rename was accepted: %v", err)
	}
}

func Test3709RenameSameSchemaRecreateCannotVerify(t *testing.T) {
	out, _, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/projects/old" {
			fmt.Fprint(w, `{"name":"old","schema_name":"physical_old","created_at":"2026-11-17T00:00:00Z","role":"owner"}`)
			return
		}
		if r.Method == "GET" && r.URL.Path == "/projects/current" {
			fmt.Fprint(w, `{"name":"current","schema_name":"physical_old","created_at":"2026-11-18T00:00:00Z","role":"owner"}`)
			return
		}
		renameHandler3709("complete")(w, r)
	}, "project", "rename", "old", "current")
	if err == nil || !strings.Contains(err.Error(), "committed") || out != "" {
		t.Fatalf("recreated physical spelling was accepted: %v %s", err, out)
	}
}
