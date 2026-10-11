package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// STOMPY-2728: `stompy context search` called the list route's lexical
// filter (GET /projects/{p}/contexts?search=) and returned [] for a query
// the REST search route and MCP both find. The contract maps both
// `context search` and `search` to GET /projects/{name}/search. These tests
// drive the REAL command tree (rootCmd.Execute) and assert the request PATH
// AND QUERY PARAMS the server actually receives, then the output decode.

type recordedReq struct {
	method string
	path   string
	query  url.Values
	body   string
}

// runCLI executes `stompy <args...>` against h and returns stdout, the error
// and every request the server saw.
func runCLI(t *testing.T, h http.HandlerFunc, args ...string) (stdout string, reqs []recordedReq, err error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("STOMPY_API_KEY", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reqs = append(reqs, recordedReq{r.Method, r.URL.EscapedPath(), r.URL.Query(), string(b)})
		h(w, r)
	}))
	defer srv.Close()

	// cobra keeps flag values (and Changed) between Execute calls in one process.
	reset := func(fs *pflag.FlagSet) {
		fs.VisitAll(func(f *pflag.Flag) {
			if sv, ok := f.Value.(pflag.SliceValue); ok {
				_ = sv.Replace(nil)
			} else {
				_ = f.Value.Set(f.DefValue)
			}
			f.Changed = false
		})
	}
	reset(rootCmd.PersistentFlags())
	for _, cmdPath := range [][]string{{"search"}, {"context", "search"}, {"init"}, {"project", "check"},
		{"project", "set-metadata"}, {"project", "link"}, {"project", "unlink"}, {"project", "links"}} {
		if c, _, ferr := rootCmd.Find(cmdPath); ferr == nil && c != nil {
			reset(c.Flags())
		}
	}

	updateAvailable = make(chan string, 1)
	full := append([]string{"--api-url", srv.URL, "--api-key", "tok"}, args...)
	stdout = captureStdout(t, func() {
		_ = captureStderr(t, func() {
			rootCmd.SetArgs(full)
			err = rootCmd.Execute()
		})
	})
	// PersistentPreRunE (which closes the channel) never runs when flag
	// parsing fails, so the drain must be bounded.
	timeout := time.After(2 * time.Second)
	for done := false; !done; {
		select {
		case _, ok := <-updateAvailable:
			done = !ok
		case <-timeout:
			done = true
		}
	}
	return
}

func replyJSON(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

const emptySearchBody = `{"results":[],"total_found":0,"search_mode":"hybrid"}`

func TestSearchRoutes_RequestPathAndParams(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want map[string]string // exact expected query params
	}{
		{
			name: "context search hits the search route with query=, not the list route's search=",
			args: []string{"-p", "proj", "context", "search", "slimy garden"},
			want: map[string]string{"query": "slimy garden", "limit": "10"},
		},
		{
			name: "top-level search sends query= (not q=)",
			args: []string{"-p", "proj", "search", "slimy garden"},
			want: map[string]string{"query": "slimy garden", "limit": "10"},
		},
		{
			name: "context search forwards every filter the route has",
			args: []string{"-p", "proj", "context", "search", "redis", "--limit", "5", "--priority", "important",
				"--tags", "cache,infra", "--scope", "project+global", "--threshold", "0.4", "--hybrid=false",
				"--chunks", "--token-budget", "5000", "--temporal", "--decay-rate", "0.05",
				"--expand-entities", "--expand-links", "--explain"},
			want: map[string]string{"query": "redis", "limit": "5", "priority": "important", "tags": "cache,infra",
				"scope": "project+global", "threshold": "0.4", "use_hybrid": "false", "search_chunks": "true",
				"token_budget": "5000", "use_temporal": "true", "temporal_decay_rate": "0.05",
				"expand_entities": "true", "expand_links": "true", "explain": "true"},
		},
		{
			name: "top-level search takes the same filters",
			args: []string{"-p", "proj", "search", "redis", "--limit", "3", "--priority", "reference", "--tags", "a",
				"--scope", "global", "--explain"},
			want: map[string]string{"query": "redis", "limit": "3", "priority": "reference", "tags": "a",
				"scope": "global", "explain": "true"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, reqs, err := runCLI(t, replyJSON(200, emptySearchBody), tc.args...)
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if len(reqs) != 1 {
				t.Fatalf("got %d requests, want exactly 1: %+v", len(reqs), reqs)
			}
			got := reqs[0]
			if got.method != "GET" || got.path != "/projects/proj/search" {
				t.Fatalf("request = %s %s, want GET /projects/proj/search", got.method, got.path)
			}
			for k, v := range tc.want {
				if g := got.query.Get(k); g != v {
					t.Errorf("param %s = %q, want %q (all: %v)", k, g, v, got.query)
				}
			}
			for k := range got.query {
				if _, ok := tc.want[k]; !ok {
					t.Errorf("unexpected param %s=%q sent (all: %v)", k, got.query.Get(k), got.query)
				}
			}
		})
	}
}

// A hit as the route emits it (recorded shape, internal/api/testdata) plus
// the optional version/lifecycle MCP shows per hit.
const searchHitsBody = `{"results":[
 {"id":7,"label":"slug_rule","version":"3","lifecycle":"active",
  "preview":"slugs pellets",
  "metadata":"{\"priority\":\"important\",\"tags\":[\"garden\"]}",
  "similarity":0.41,"rrf_score":0.033,"source":"hybrid","scope":"project"},
 {"id":9,"label":"old_rule","version":"1","lifecycle":"archived",
  "preview":"poison","metadata":"{\"priority\":\"reference\"}",
  "bm25_score":0.2,"rrf_score":0.016,"source":"bm25","scope":"project"}
],"total_found":2,"vector_count":1,"bm25_count":1,"overlap_count":0,"search_mode":"hybrid"}`

func TestSearchRoutes_RendersWhatMCPReturnsPerHit(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "proj", "context", "search", "slimy"},
		{"-p", "proj", "search", "slimy"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			out, _, err := runCLI(t, replyJSON(200, searchHitsBody), args...)
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			for _, want := range []string{
				"slug_rule", "old_rule", // label/topic
				"0.0330",         // score (rrf)
				"hybrid", "bm25", // source
				"important", "reference", // priority
				"slugs",                // preview
				"archived",             // lifecycle is labelled
				"VERSION", "LIFECYCLE", // columns exist
				"2 results", "hybrid", // footer names count and mode
			} {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
		})
	}
}

func TestSearchRoutes_JSONIsTheAPIObject(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "proj", "-o", "json", "context", "search", "slimy"},
		{"-p", "proj", "-o", "json", "search", "slimy"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			out, _, err := runCLI(t, replyJSON(200, searchHitsBody), args...)
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatalf("stdout is not one JSON object: %v\n%s", err, out)
			}
			var want map[string]any
			_ = json.Unmarshal([]byte(searchHitsBody), &want)
			// vector_count is a field the CLI struct does not model: it only
			// survives if the API object is passed through verbatim.
			if got["vector_count"] != want["vector_count"] || got["search_mode"] != "hybrid" {
				t.Errorf("not the API object: %v", got)
			}
			if rs, _ := got["results"].([]any); len(rs) != 2 {
				t.Errorf("results = %v, want 2 hits", got["results"])
			}
		})
	}
}

func TestSearchRoutes_EmptyIsExplicitAndNamesTheMode(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "proj", "context", "search", "nothing here"},
		{"-p", "proj", "search", "nothing here"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			out, _, err := runCLI(t, replyJSON(200, `{"results":[],"total_found":0,"search_mode":"vector_only"}`), args...)
			if err != nil {
				t.Fatalf("Execute() error: %v", err)
			}
			if !strings.Contains(out, "no matches") || !strings.Contains(out, "vector_only") || !strings.Contains(out, "nothing here") {
				t.Errorf("empty result must say 'no matches' with the query and search_mode:\n%s", out)
			}
		})
	}
}

func TestSearchRoutes_ServerErrorIsAnErrorNotAnEmptyList(t *testing.T) {
	for _, args := range [][]string{
		{"-p", "proj", "context", "search", "q"},
		{"-p", "proj", "search", "q"},
	} {
		t.Run(strings.Join(args[2:], " "), func(t *testing.T) {
			out, _, err := runCLI(t, replyJSON(422, `{"detail":[{"loc":["query","query"],"msg":"Field required"}]}`), args...)
			if err == nil {
				t.Fatalf("a 422 must surface as an error, got none; stdout=%q", out)
			}
			if strings.Contains(out, "no matches") {
				t.Errorf("an error must not read as 'no matches': %q", out)
			}
		})
	}
}
