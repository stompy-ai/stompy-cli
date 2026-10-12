package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// STOMPY-2712: `project set-metadata|link|unlink|links` — the CLI door for
// project.set_metadata, project.link, project.unlink, project.edges.

func TestSetMetadata_PatchesGivenFields(t *testing.T) {
	out, reqs, err := runCLI(t, replyJSON(200, `{"project":"proj","schema":"s_1","kg":{"origin":"agent","category":null,"lifecycle_state":null,"purpose":"ship it","intent":null}}`),
		"project", "set-metadata", "proj", "--origin", "agent", "--purpose", "ship it", "--clear", "intent,category")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != http.MethodPatch || reqs[0].path != "/projects/proj" {
		t.Fatalf("reqs = %+v", reqs)
	}
	var got map[string]any
	if jerr := json.Unmarshal([]byte(reqs[0].body), &got); jerr != nil {
		t.Fatalf("body %q: %v", reqs[0].body, jerr)
	}
	if got["origin"] != "agent" || got["purpose"] != "ship it" {
		t.Errorf("body = %v", got)
	}
	if c, _ := got["clear"].([]any); len(c) != 2 {
		t.Errorf("clear = %v", got["clear"])
	}
	if _, has := got["lifecycle_state"]; has {
		t.Errorf("unset flag leaked into body: %v", got)
	}
	if !strings.Contains(out, "ship it") || !strings.Contains(out, "agent") {
		t.Errorf("table must show the kg values read back: %q", out)
	}
}

func TestSetMetadata_JSONIsTheAPIObject(t *testing.T) {
	out, _, err := runCLI(t, replyJSON(200, `{"project":"proj","schema":"s_1","kg":{"origin":"human"},"zz_unmodelled":true}`),
		"-o", "json", "project", "set-metadata", "proj", "--origin", "human")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if jerr := json.Unmarshal([]byte(out), &got); jerr != nil {
		t.Fatalf("not JSON: %q", out)
	}
	if got["zz_unmodelled"] != true || got["schema"] != "s_1" {
		t.Errorf("-o json must print the API object, got %v", got)
	}
}

func TestSetMetadata_FallsBackToResolvedProject(t *testing.T) {
	_, reqs, err := runCLI(t, replyJSON(200, `{"project":"proj","kg":{}}`), "-p", "proj", "project", "set-metadata", "--purpose", "x")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].path != "/projects/proj" {
		t.Errorf("reqs = %+v", reqs)
	}
}

const foreignRefusal = "Refused: '@alice/proj' is in another owner's namespace, not yours. project_set_metadata acts on personal projects only (your own), so nothing was changed."

func TestRefusalPassesThroughIntactAndNothingIsRetried(t *testing.T) {
	for _, args := range [][]string{
		{"project", "set-metadata", "@alice/proj", "--purpose", "x"},
		{"project", "link", "@alice/proj", "other", "--relation-type", "feeds"},
		{"project", "unlink", "@alice/proj", "other"},
	} {
		t.Run(args[1], func(t *testing.T) {
			_, reqs, err := runCLI(t, replyJSON(403, `{"detail":"`+foreignRefusal+`"}`), args...)
			if err == nil || !strings.Contains(err.Error(), foreignRefusal) {
				t.Fatalf("refusal text must reach the user verbatim, got %v", err)
			}
			if len(reqs) != 1 {
				t.Fatalf("CLI must not work around a refusal: %d requests %+v", len(reqs), reqs)
			}
			if reqs[0].path != "/projects/proj/edges" && reqs[0].path != "/projects/proj" {
				t.Errorf("path = %s", reqs[0].path)
			}
			if reqs[0].query.Get("owner") != "alice" {
				t.Errorf("the qualified address must be sent as ?owner=, got %v", reqs[0].query)
			}
		})
	}
}

func TestLink_PostsEdgeAndPrintsIt(t *testing.T) {
	out, reqs, err := runCLI(t, replyJSON(201, `{"project":"a","edge":{"source":"a","target":"b","relation_type":"feeds","note":"n"}}`),
		"project", "link", "a", "b", "--relation-type", "feeds", "--note", "n")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != "POST" || reqs[0].path != "/projects/a/edges" {
		t.Fatalf("reqs = %+v", reqs)
	}
	if !strings.Contains(out, "feeds") || !strings.Contains(out, "b") {
		t.Errorf("out = %q", out)
	}
}

func TestLink_SourceFromResolvedProjectWhenOneArg(t *testing.T) {
	_, reqs, err := runCLI(t, replyJSON(201, `{"project":"a","edge":{}}`),
		"-p", "a", "project", "link", "b", "--relation-type", "feeds")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].path != "/projects/a/edges" {
		t.Errorf("reqs = %+v", reqs)
	}
}

func TestLink_RequiresRelationType(t *testing.T) {
	_, reqs, err := runCLI(t, replyJSON(201, `{}`), "project", "link", "a", "b")
	if err == nil || len(reqs) != 0 {
		t.Fatalf("err=%v reqs=%d: relation-type is required and nothing is sent without it", err, len(reqs))
	}
}

func TestUnlink_DeletesAndPrintsAPIObject(t *testing.T) {
	out, reqs, err := runCLI(t, replyJSON(200, `{"project":"a","target":"s_b","removed":1}`),
		"-o", "json", "project", "unlink", "a", "b", "--relation-type", "feeds")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != "DELETE" || reqs[0].path != "/projects/a/edges" ||
		reqs[0].query.Get("target") != "b" || reqs[0].query.Get("relation_type") != "feeds" {
		t.Fatalf("reqs = %+v", reqs)
	}
	var got map[string]any
	if json.Unmarshal([]byte(out), &got) != nil || got["removed"] != float64(1) || got["target"] != "s_b" {
		t.Errorf("-o json must print the API object, got %q", out)
	}
}

func TestLinks_TableAndJSON(t *testing.T) {
	body := `{"project":"a","schema":"s_a","edges":[{"source":"a","target":"b","relation_type":"feeds","note":"n","direction":"out"}]}`
	out, reqs, err := runCLI(t, replyJSON(200, body), "project", "links", "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != "GET" || reqs[0].path != "/projects/a/edges" {
		t.Fatalf("reqs = %+v", reqs)
	}
	if !strings.Contains(out, "feeds") {
		t.Errorf("table = %q", out)
	}
	jout, _, _ := runCLI(t, replyJSON(200, body), "-o", "json", "project", "links", "a")
	var got map[string]any
	if json.Unmarshal([]byte(jout), &got) != nil || got["schema"] != "s_a" {
		t.Errorf("-o json = %q", jout)
	}
}
