package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const marker3709 = "version: 1\nenvironment: production\nprojects:\n  - address: \"@alice/old\"\n    role: shared\n    default: true\n"

func alias3709(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"name":"current","schema_name":"physical_old","role":"contributor","renamed_from":"@alice/old","now_called":"@alice/current"}`)
}

func Test3709CheckRenamedPreservesMarkerAndScope(t *testing.T) {
	dir := repoDir(t, marker3709)
	out, reqs, err := runCLI(t, alias3709, "project", "check")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"RENAMED", "@alice/old", `address: "@alice/current"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q: %s", want, out)
		}
	}
	if len(reqs) != 1 || reqs[0].query.Get("owner") != "alice" {
		t.Fatalf("wrong scope: %+v", reqs)
	}
	b, err := os.ReadFile(filepath.Join(dir, ".stompy.yaml"))
	if err != nil || string(b) != marker3709 {
		t.Fatalf("marker modified: %s %v", b, err)
	}
}

func Test3709CheckRenamedJSON(t *testing.T) {
	repoDir(t, marker3709)
	out, _, err := runCLI(t, alias3709, "-o", "json", "project", "check")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["status"] != "renamed" || rows[0]["now_called"] != "@alice/current" || rows[0]["marker_line"] != `  - address: "@alice/current"` {
		t.Fatalf("missing actionable machine result: %s", out)
	}
}

func Test3709CheckRenamedDoesNotHideMissing(t *testing.T) {
	repoDir(t, marker3709+"  - address: missing\n    role: reference\n")
	out, _, err := runCLI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/missing") {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"detail":"missing"}`)
			return
		}
		alias3709(w, r)
	}, "project", "check")
	if exitCode(err) != 1 || !strings.Contains(out, "RENAMED") || !strings.Contains(out, "NOT FOUND") {
		t.Fatalf("wrong mixed verdict: %v %s", err, out)
	}
}

func Test3709CheckCanonicalControl(t *testing.T) {
	repoDir(t, marker3709)
	out, _, err := runCLI(t, projectHandler("", ""), "project", "check")
	if err != nil || !strings.Contains(out, "OK") || strings.Contains(out, "address:") {
		t.Fatalf("canonical changed: %v %s", err, out)
	}
}

func Test3709CheckUnverifiableNoticeDoesNotSuggestAddress(t *testing.T) {
	for _, body := range []string{
		`{"name":"current","renamed_from":"@alice/other","now_called":"@alice/current"}`,
		`{"name":"current","renamed_from":"@alice/old","now_called":"@mallory/current"}`,
		`{"name":"current","renamed_from":"@alice/old","now_called":"current"}`,
		`{"name":"current","renamed_from":"@alice/old","now_called":"@alice/later"}`,
		`{"name":"current","renamed_from":"@alice/old","now_called":"@alice/current","enrichment_warning":"PROJECT_DISPLAY_UNAVAILABLE"}`,
	} {
		repoDir(t, marker3709)
		out, _, err := runCLI(t, replyJSON(200, body), "project", "check")
		if exitCode(err) != 1 || strings.Contains(out, "address:") || !strings.Contains(out, "ERROR") {
			t.Fatalf("must refuse an invented correction: %v %s", err, out)
		}
	}
}

func Test3709CheckPersonalCorrection(t *testing.T) {
	repoDir(t, strings.ReplaceAll(marker3709, "@alice/old", "old"))
	out, _, err := runCLI(t, replyJSON(200, `{"name":"current","renamed_from":"old","now_called":"current"}`), "project", "check")
	if err != nil || !strings.Contains(out, `address: "current"`) {
		t.Fatalf("wrong personal correction: %v %s", err, out)
	}
}
