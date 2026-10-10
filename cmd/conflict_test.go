package cmd

import (
	"strings"
	"testing"

	"github.com/banton/stompy-cli/internal/api"
)

// STOMPY-1814: stompy conflict list --status unresolved|resolved|dismissed
// (the CLI's old documented vocabulary) silently returned zero rows against
// both prod and staging, because the server's actual conflict statuses
// since the 1767 A-train are pending/user_resolved/auto_resolved/dismissed.
// validateConflictStatus must reject an unrecognized value client-side,
// naming the real vocabulary, instead of letting it reach the API.
func TestValidateConflictStatus(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		wantErr bool
	}{
		{name: "empty status is not filtered", status: "", wantErr: false},
		{name: "pending is valid", status: "pending", wantErr: false},
		{name: "user_resolved is valid", status: "user_resolved", wantErr: false},
		{name: "auto_resolved is valid", status: "auto_resolved", wantErr: false},
		{name: "dismissed is valid", status: "dismissed", wantErr: false},
		{name: "valid value is case-insensitive", status: "PENDING", wantErr: false},
		{
			name:    "the drifted CLI vocabulary value unresolved is rejected",
			status:  "unresolved",
			wantErr: true,
		},
		{
			name:    "the drifted CLI vocabulary value resolved is rejected",
			status:  "resolved",
			wantErr: true,
		},
		{name: "an arbitrary unknown value is rejected", status: "bogus", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConflictStatus(tc.status)
			if tc.wantErr && err == nil {
				t.Fatalf("validateConflictStatus(%q) = nil, want an error", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateConflictStatus(%q) = %v, want nil", tc.status, err)
			}
		})
	}
}

func TestValidateConflictStatusErrorNamesVocabulary(t *testing.T) {
	err := validateConflictStatus("unresolved")
	if err == nil {
		t.Fatal("expected an error for an unknown status")
	}
	for _, want := range []string{"pending", "user_resolved", "auto_resolved", "dismissed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name valid value %q", err.Error(), want)
		}
	}
}

// STOMPY-2663: keep_a / keep_b ARCHIVES the rejected side's topic. The server
// says so in `archived_context`; the CLI must not hide it behind a bare
// "resolved (user_resolved)", and its help must state the semantics.
func TestResolveSummaryNamesTheArchivedTopicAndUndo(t *testing.T) {
	resp := &api.ConflictResponse{
		ID:     "c-1",
		Status: "user_resolved",
		ArchivedContext: &api.ArchivedContext{
			Archived: true, Topic: "refund_window_b", Version: "1.1",
			KeptTopic: "refund_window_a",
			Undo:      `set_lifecycle(topic="refund_window_b", state="active")`,
		},
	}
	got := resolveSummary(resp)
	for _, want := range []string{"refund_window_b", "1.1", "set_lifecycle", "refund_window_a"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q does not contain %q", got, want)
		}
	}
}

func TestResolveSummaryExplainsWhyNothingWasArchived(t *testing.T) {
	resp := &api.ConflictResponse{
		ID: "c-1", Status: "user_resolved",
		ArchivedContext: &api.ArchivedContext{Archived: false, Reason: "same_topic"},
	}
	if got := resolveSummary(resp); !strings.Contains(got, "same_topic") {
		t.Errorf("summary %q does not say why nothing was archived", got)
	}
}

func TestResolveSummaryForDismissIsUnchanged(t *testing.T) {
	resp := &api.ConflictResponse{ID: "c-1", Status: "dismissed"}
	got := resolveSummary(resp)
	if strings.Contains(strings.ToLower(got), "archiv") {
		t.Errorf("a dismiss must not mention archiving, got %q", got)
	}
}

func TestResolveHelpStatesTheArchiveSemantics(t *testing.T) {
	usage := conflictResolveCmd.Flags().Lookup("resolution").Usage
	for _, want := range []string{"keep_a", "keep_b", "archive", "dismiss"} {
		if !strings.Contains(strings.ToLower(usage), want) {
			t.Errorf("--resolution help %q does not mention %q", usage, want)
		}
	}
}
