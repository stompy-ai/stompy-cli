package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// Recorded shapes of the server's board answers after STOMPY-3172/3178/3182
// (dementia-production src/services/board_service.py, board_authors.py).
const boardPostAnswer = `{"project":"mine","agent_label":"",
 "superseded_other_session":true,
 "notice":"This status replaced a live status under the same agent_label, and this door cannot tell its writers apart.",
 "post":{"id":7,"author_id":101,"agent_label":"","kind":"status","body":"working","refs":[],
  "created_at":"2026-10-30T10:00:00Z","expires_at":"2026-10-31T10:00:00Z","superseded_at":null,
  "read_count":0,"last_read_at":null,"delivered_count":0,"viewed_count":0,"last_viewed_at":null,
  "author_kind":"agent","weight":1.0,"author_name":"noisy-sea-11911466","author_role":null,"is_caller":true}}`

const boardReadAnswer = `{"project":"mine","posts":[
 {"id":8,"author_id":202,"agent_label":"Claude","kind":"message","body":"hi","refs":[],
  "created_at":"2026-10-30T10:00:00Z","expires_at":"2026-11-06T10:00:00Z","read_count":3,
  "delivered_count":2,"viewed_count":1,"last_viewed_at":"2026-10-30T11:00:00Z",
  "author_kind":"human","weight":0.9,"author_name":"Markus","author_role":"owner","is_caller":false}]}`

func runBoardAgainst(t *testing.T, answer, format, sub string, flags ...string) (string, string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, answer)
	}))
	defer srv.Close()
	oldClient, oldProject := apiClient, flagProject
	apiClient, flagProject = api.NewClient(srv.URL, "tok", "dev", false), "mine"
	defer func() { apiClient, flagProject = oldClient, oldProject }()
	command, _, err := newBoardCmd().Find([]string{sub})
	if err != nil {
		t.Fatal(err)
	}
	if err = command.ParseFlags(flags); err != nil {
		t.Fatal(err)
	}
	var out, errOut string
	withOutputFormat(format, func() {
		errOut = captureStderr(t, func() {
			out = captureStdout(t, func() {
				if err = command.RunE(command, nil); err != nil {
					t.Error(err)
				}
			})
		})
	})
	return out, errOut
}

func decodeMap(t *testing.T, out string) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	return got
}

func TestBoardPostJSONKeepsAuthorAndSupersedeFields(t *testing.T) {
	out, _ := runBoardAgainst(t, boardPostAnswer, "json", "post", "--body", "working")
	got := decodeMap(t, out)
	if got["superseded_other_session"] != true || !strings.Contains(got["notice"].(string), "cannot tell its writers apart") {
		t.Errorf("supersede notice dropped: %s", out)
	}
	if label, ok := got["agent_label"]; !ok || label != "" {
		t.Errorf("answer agent_label dropped: %s", out)
	}
	post := got["post"].(map[string]any)
	want := map[string]any{"author_kind": "agent", "author_name": "noisy-sea-11911466", "is_caller": true, "delivered_count": float64(0), "viewed_count": float64(0)}
	for key, value := range want {
		if post[key] != value {
			t.Errorf("post.%s = %#v, want %#v", key, post[key], value)
		}
	}
	if role, ok := post["author_role"]; !ok || role != nil {
		t.Errorf("post.author_role must pass through as null, got %#v (present=%v)", role, ok)
	}
}

func TestBoardReadJSONKeepsAuthorFields(t *testing.T) {
	out, _ := runBoardAgainst(t, boardReadAnswer, "json", "read")
	post := decodeMap(t, out)["posts"].([]any)[0].(map[string]any)
	want := map[string]any{"author_kind": "human", "author_name": "Markus", "author_role": "owner", "is_caller": false, "delivered_count": float64(2), "viewed_count": float64(1), "last_viewed_at": "2026-10-30T11:00:00Z"}
	for key, value := range want {
		if post[key] != value {
			t.Errorf("post.%s = %#v, want %#v", key, post[key], value)
		}
	}
}

func TestBoardReadTableShowsAuthor(t *testing.T) {
	out, _ := runBoardAgainst(t, boardReadAnswer, "table", "read")
	if !strings.Contains(out, `by "Markus" (owner, human)`) {
		t.Fatalf("author not shown: %q", out)
	}
}

func TestBoardPostTableShowsCallerAndNotice(t *testing.T) {
	out, errOut := runBoardAgainst(t, boardPostAnswer, "table", "post", "--body", "working")
	if !strings.Contains(out, `by "noisy-sea-11911466" (agent, you)`) {
		t.Errorf("author not shown: %q", out)
	}
	if !strings.Contains(errOut, "cannot tell its writers apart") {
		t.Errorf("supersede notice not on stderr: %q", errOut)
	}
}

func TestBoardReadTableOlderServerHasNoAuthorSuffix(t *testing.T) {
	old := `{"project":"mine","posts":[{"id":9,"author_id":5,"agent_label":"a","kind":"status","body":"b","refs":[],"created_at":"2026-10-30T10:00:00Z","expires_at":"2026-10-31T10:00:00Z","read_count":0,"weight":1}]}`
	out, _ := runBoardAgainst(t, old, "table", "read")
	if !strings.Contains(out, "#9 account:5") || strings.Contains(out, " by ") {
		t.Fatalf("older server rendering changed: %q", out)
	}
}
