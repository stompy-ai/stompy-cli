package cmd

import (
	"github.com/spf13/cobra"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestCLI3859_UnknownFlagReturnsPromptly(t *testing.T) {
	if !lifecycle3858Child(t) {
		return
	}
	out, requests, err := runCLI(t, replyJSON(200, `{}`), "context", "list", "--missing-3859")
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --missing-3859") || out != "" || len(requests) != 0 {
		t.Fatalf("early error: output=%q requests=%v error=%v", out, requests, err)
	}
}
func TestCLI3859_ContextListFlagsDoNotLeak(t *testing.T) {
	if !lifecycle3858Child(t) {
		return
	}
	for i, extra := range [][]string{{"--limit", "3", "--priority", "important", "--fresh"}, nil} {
		args := append([]string{"-p", "proj", "-o", "json", "context", "list"}, extra...)
		_, requests, err := runCLI(t, replyJSON(200, `{"contexts":[],"total":0}`), args...)
		if err != nil {
			t.Fatal(err)
		}
		if len(requests) != 1 {
			t.Fatalf("requests=%v", requests)
		}
		want := url.Values{}
		if i == 0 {
			want = url.Values{"limit": {"3"}, "priority": {"important"}}
		}
		if !reflect.DeepEqual(requests[0].query, want) {
			t.Errorf("call%d query=%v want%v", i, requests[0].query, want)
		}
		if contextListCmd.Flags().Changed("fresh") != (i == 0) {
			t.Errorf("call%d retained --fresh Changed", i)
		}
	}
}
func TestCLI3859_DeepLocalAndPersistentFlagsDoNotLeak(t *testing.T) {
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
		if _, _, err := runCLI(t, jsonReply(200, `{}`), args...); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("executed %d probes, want both invocations", calls)
	}
}

func TestCLI3859_SliceFlagsDoNotLeak(t *testing.T) {
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
		if _, _, err := runCLI(t, jsonReply(200, `{}`), append([]string{"slices-3858"}, extra...)...); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("executed %d probes, want both invocations", calls)
	}
}
