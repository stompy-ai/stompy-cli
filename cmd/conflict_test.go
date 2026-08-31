package cmd

import (
	"strings"
	"testing"
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
