package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func Test3709AliasesQualifiedSortedAndCurrentName(t *testing.T) {
	out, reqs, err := runCLI(t, replyJSON(200, `{"name":"current","schema_name":"physical","aliases":["zebra","before"]}`), "-o", "json", "project", "aliases", "@alice/current")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].path != "/projects/current/aliases" || reqs[0].query.Get("owner") != "alice" {
		t.Fatal(reqs)
	}
	var body struct {
		Name    string   `json:"name"`
		Aliases []string `json:"aliases"`
	}
	if json.Unmarshal([]byte(out), &body) != nil || body.Name != "current" || strings.Join(body.Aliases, ",") != "before,zebra" {
		t.Fatalf("not a current sorted list: %s", out)
	}
}

func Test3709AliasesRefusalNotEmpty(t *testing.T) {
	for _, status := range []int{403, 404} {
		out, reqs, err := runCLI(t, replyJSON(status, `{"error_code":"RENAME_FORBIDDEN","detail":"Owner only"}`), "project", "aliases", "current")
		if err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) || out != "" || len(reqs) != 1 {
			t.Fatalf("refusal masked: %v %q %+v", err, out, reqs)
		}
	}
}

func Test3709AliasesMissingEvidenceNotEmpty(t *testing.T) {
	out, _, err := runCLI(t, replyJSON(200, `{"name":"current","schema_name":"physical"}`), "project", "aliases", "current")
	if err == nil || out != "" {
		t.Fatalf("unknown list rendered empty: %v %s", err, out)
	}
}

func Test3709AliasesEmptyIsExplicit(t *testing.T) {
	out, _, err := runCLI(t, replyJSON(200, `{"name":"current","schema_name":"physical","aliases":[]}`), "project", "aliases", "current")
	if err != nil || !strings.Contains(out, "current") || !strings.Contains(out, "none") {
		t.Fatalf("confirmed empty list missing: %v %s", err, out)
	}
}

func Test3709AliasesArityNoNetwork(t *testing.T) {
	for _, args := range [][]string{{}, {"a", "b"}, {""}} {
		_, reqs, err := runCLI(t, replyJSON(200, `{}`), append([]string{"project", "aliases"}, args...)...)
		if err == nil || len(reqs) != 0 {
			t.Fatalf("bad arity %v: %v %+v", args, err, reqs)
		}
	}
}
