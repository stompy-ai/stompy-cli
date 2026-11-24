package cmd

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// Isolate global Cobra state and bound the broken harness without leaving a
// stuck goroutine behind. A regression is a named failure, not go test's timeout.
func lifecycle3858Child(t *testing.T) bool {
	t.Helper()
	if os.Getenv("STOMPY_3858_CHILD") == t.Name() {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.v")
	command.Env = append(os.Environ(), "STOMPY_3858_CHILD="+t.Name())
	out, err := command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("runLifecycle did not return within one second: %s", out)
	}
	if err != nil {
		t.Fatalf("runLifecycle child failed: %v\n%s", err, out)
	}
	return false
}

func lifecycle3858Reject(t *testing.T, want string, args ...string) {
	t.Helper()
	out, stderr, call, err := runLifecycle(t, jsonReply(200, `{}`), args...)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
	if out != "" || stderr != "" || call.method != "" {
		t.Fatalf("early error produced output or HTTP: %q, %q, %+v", out, stderr, call)
	}
}

func TestLifecycle3858_UnknownFlagReturnsPromptly(t *testing.T) {
	if lifecycle3858Child(t) {
		lifecycle3858Reject(t, "unknown flag: --missing-3858", "context", "lifecycle", "topic", "--missing-3858")
	}
}

func TestLifecycle3858_InvalidFlagValueReturnsPromptly(t *testing.T) {
	if lifecycle3858Child(t) {
		lifecycle3858Reject(t, "invalid argument", "context", "lifecycle", "topic", "--review-after-days", "not-a-number")
	}
}

func TestLifecycle3858_PreRunErrorReturnsPromptly(t *testing.T) {
	if lifecycle3858Child(t) {
		lifecycle3858Reject(t, "unknown output format", "--output", "invalid-3858", "search", "topic")
	}
}

func lifecycle3858Search(t *testing.T, path ...string) {
	t.Helper()
	var queries []url.Values
	reply := func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query())
		jsonReply(200, emptySearchBody)(w, r)
	}
	base := append([]string{"-p", "proj", "-o", "json"}, path...)
	first := append(append([]string{}, base...), "oracle", "--limit", "3", "--priority", "important", "--hybrid=false")
	if _, _, _, err := runLifecycle(t, reply, first...); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := runLifecycle(t, reply, append(base, "oracle")...); err != nil {
		t.Fatal(err)
	}
	want := []url.Values{
		{"query": {"oracle"}, "limit": {"3"}, "priority": {"important"}, "use_hybrid": {"false"}},
		{"query": {"oracle"}, "limit": {"10"}},
	}
	if !reflect.DeepEqual(queries, want) {
		t.Fatalf("requests retained values or Changed bits: got %v, want %v", queries, want)
	}
}

func TestLifecycle3858_TopLevelSearchFlagsDoNotLeak(t *testing.T) {
	if lifecycle3858Child(t) {
		lifecycle3858Search(t, "search")
	}
}

func TestLifecycle3858_NestedSearchFlagsDoNotLeak(t *testing.T) {
	if lifecycle3858Child(t) {
		lifecycle3858Search(t, "context", "search")
	}
}

func TestLifecycle3858_DeepLocalAndPersistentFlagsDoNotLeak(t *testing.T) {
	if !lifecycle3858Child(t) {
		return
	}
	parent := &cobra.Command{Use: "harness-3858"}
	group := &cobra.Command{Use: "nested"}
	leaf := &cobra.Command{Use: "probe"}
	parent.PersistentFlags().String("inherited-3858", "default", "test flag")
	leaf.Flags().Bool("local-3858", false, "test flag")
	group.AddCommand(leaf)
	parent.AddCommand(group)
	rootCmd.AddCommand(parent)
	defer rootCmd.RemoveCommand(parent)
	calls := 0
	leaf.RunE = func(command *cobra.Command, args []string) error {
		inherited, _ := command.Flags().GetString("inherited-3858")
		local, _ := command.Flags().GetBool("local-3858")
		wantChanged := calls == 0
		if inherited != map[bool]string{true: "stale", false: "default"}[wantChanged] || local != wantChanged ||
			command.Flags().Changed("inherited-3858") != wantChanged || command.Flags().Changed("local-3858") != wantChanged {
			t.Errorf("call %d: inherited=%q/local=%v, Changed=%v/%v", calls, inherited, local,
				command.Flags().Changed("inherited-3858"), command.Flags().Changed("local-3858"))
		}
		calls++
		return nil
	}
	for _, extra := range [][]string{{"--inherited-3858", "stale", "--local-3858"}, nil} {
		args := append([]string{"harness-3858", "nested", "probe"}, extra...)
		if _, _, _, err := runLifecycle(t, jsonReply(200, `{}`), args...); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("executed %d probes, want both invocations", calls)
	}
}

func TestLifecycle3858_SliceFlagsDoNotLeak(t *testing.T) {
	if !lifecycle3858Child(t) {
		return
	}
	probe := &cobra.Command{Use: "slices-3858"}
	probe.Flags().StringSlice("topics-3858", nil, "test flag")
	probe.Flags().StringArray("refs-3858", nil, "test flag")
	rootCmd.AddCommand(probe)
	defer rootCmd.RemoveCommand(probe)
	calls := 0
	probe.RunE = func(command *cobra.Command, args []string) error {
		topics, _ := command.Flags().GetStringSlice("topics-3858")
		refs, _ := command.Flags().GetStringArray("refs-3858")
		if calls == 0 {
			if !reflect.DeepEqual(topics, []string{"a", "b"}) || !reflect.DeepEqual(refs, []string{"first", "second"}) {
				t.Errorf("explicit lists = %v/%v", topics, refs)
			}
		} else if len(topics) != 0 || len(refs) != 0 || command.Flags().Changed("topics-3858") || command.Flags().Changed("refs-3858") {
			t.Errorf("lists or Changed bits leaked: %v/%v", topics, refs)
		}
		calls++
		return nil
	}
	for _, extra := range [][]string{{"--topics-3858", "a,b", "--refs-3858", "first", "--refs-3858", "second"}, nil} {
		if _, _, _, err := runLifecycle(t, jsonReply(200, `{}`), append([]string{"slices-3858"}, extra...)...); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("executed %d probes, want both invocations", calls)
	}
}
