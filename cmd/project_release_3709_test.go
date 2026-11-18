package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func releaseHandler3709(mode string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "DELETE":
			if mode == "refused" {
				w.WriteHeader(403)
				fmt.Fprint(w, `{"error_code":"RENAME_FORBIDDEN","detail":"Owner only"}`)
				return
			}
			fmt.Fprint(w, `{"success":true,"name":"current","schema_name":"physical","released_alias":"old"}`)
		case strings.HasSuffix(r.URL.Path, "/aliases"):
			if mode == "retained" {
				fmt.Fprint(w, `{"name":"current","schema_name":"physical","aliases":["old"]}`)
				return
			}
			fmt.Fprint(w, `{"name":"current","schema_name":"physical","aliases":[]}`)
		default:
			fmt.Fprint(w, `{"name":"current","schema_name":"physical","created_at":"2026-11-17T00:00:00Z","role":"owner"}`)
		}
	}
}

func runRelease3709(t *testing.T, h http.HandlerFunc, args ...string) (string, []recordedReq, error) {
	t.Helper()
	// Own new flag, reset without editing the foreign CLI test harness.
	if c, _, err := rootCmd.Find([]string{"project", "release-name"}); err == nil {
		if f := c.Flags().Lookup("confirm"); f != nil {
			_ = f.Value.Set("false")
			f.Changed = false
		}
	}
	return runCLI(t, h, args...)
}

func Test3709ReleaseQualifiedRoundTrip(t *testing.T) {
	out, reqs, err := runRelease3709(t, releaseHandler3709("complete"), "-p", "@alice/current", "-o", "json", "project", "release-name", "old", "--confirm")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /projects/current", "DELETE /projects/current/aliases/old", "GET /projects/current", "GET /projects/current/aliases"}
	if len(reqs) != len(want) {
		t.Fatalf("readbacks missing: %+v", reqs)
	}
	for i, r := range reqs {
		if r.method+" "+r.path != want[i] || r.query.Get("owner") != "alice" {
			t.Errorf("wrong scope/order: %+v", reqs)
		}
	}
	if !strings.Contains(out, `"released_alias": "old"`) || !strings.Contains(out, `"verified": true`) {
		t.Fatalf("missing verified release: %s", out)
	}
}

func Test3709ReleaseNeedsConfirmationBeforeNetwork(t *testing.T) {
	_, reqs, err := runRelease3709(t, releaseHandler3709("complete"), "-p", "current", "project", "release-name", "old")
	if err == nil || !strings.Contains(err.Error(), "old links stop") || len(reqs) != 0 {
		t.Fatalf("missing confirmation: %v %+v", err, reqs)
	}
}

func Test3709ReleaseRetainedAliasCannotVerify(t *testing.T) {
	out, reqs, err := runRelease3709(t, releaseHandler3709("retained"), "-p", "current", "project", "release-name", "old", "--confirm")
	if err == nil || !strings.Contains(err.Error(), "unverified") || out != "" {
		t.Fatalf("retained old link accepted: %v %s", err, out)
	}
	deletes := 0
	for _, r := range reqs {
		if r.method == "DELETE" {
			deletes++
		}
	}
	if deletes != 1 {
		t.Fatal(reqs)
	}
}

func Test3709ReleaseRefusalControl(t *testing.T) {
	out, reqs, err := runRelease3709(t, releaseHandler3709("refused"), "-p", "current", "project", "release-name", "old", "--confirm")
	if err == nil || !strings.Contains(err.Error(), "RENAME_FORBIDDEN") || out != "" || len(reqs) != 2 {
		t.Fatalf("refusal masked or retried: %v %s %+v", err, out, reqs)
	}
}

func Test3709ReleaseEmptyProjectNoNetwork(t *testing.T) {
	_, reqs, err := runRelease3709(t, releaseHandler3709("complete"), "--project=", "project", "release-name", "old", "--confirm")
	if err == nil || len(reqs) != 0 {
		t.Fatalf("empty binding sent: %v %+v", err, reqs)
	}
}

func Test3709ReleaseRejectsChangedReadback(t *testing.T) {
	for _, mode := range []string{"schema", "generation", "name", "aliases"} {
		t.Run(mode, func(t *testing.T) {
			reads := 0
			out, reqs, err := runRelease3709(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && !strings.HasSuffix(r.URL.Path, "/aliases") {
					reads++
					if reads > 1 {
						schema, date, name := "physical", "2026-11-17T00:00:00Z", "current"
						if mode == "schema" {
							schema = "another"
						}
						if mode == "generation" {
							date = "2026-11-18T00:00:00Z"
						}
						if mode == "name" {
							name = "later"
						}
						fmt.Fprintf(w, `{"name":%q,"schema_name":%q,"created_at":%q}`, name, schema, date)
						return
					}
				}
				if mode == "aliases" && strings.HasSuffix(r.URL.Path, "/aliases") {
					fmt.Fprint(w, `{"name":"current","schema_name":"another","aliases":[]}`)
					return
				}
				releaseHandler3709("complete")(w, r)
			}, "-p", "current", "project", "release-name", "old", "--confirm")
			if err == nil || !strings.Contains(err.Error(), "unverified") || out != "" {
				t.Fatalf("changed %s accepted: %v %s", mode, err, out)
			}
			deletes := 0
			for _, r := range reqs {
				if r.method == "DELETE" {
					deletes++
				}
			}
			if deletes != 1 {
				t.Fatal(reqs)
			}
		})
	}
}

func Test3709ReleaseUsesMarkerBindingWithoutRewriting(t *testing.T) {
	dir := repoDir(t, strings.ReplaceAll(marker3709, "@alice/old", "@alice/current"))
	_, reqs, err := runRelease3709(t, releaseHandler3709("complete"), "project", "release-name", "old", "--confirm")
	if err != nil || len(reqs) != 4 || reqs[0].query.Get("owner") != "alice" {
		t.Fatalf("binding ignored: %v %+v", err, reqs)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".stompy.yaml"))
	if err != nil || string(b) != strings.ReplaceAll(marker3709, "@alice/old", "@alice/current") {
		t.Fatalf("marker mutated: %s %v", b, err)
	}
}
