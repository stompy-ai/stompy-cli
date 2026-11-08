package config

import (
	"strings"
	"testing"
)

// STOMPY-3499: `-p ""` passed explicitly must be refused, never read as "no flag".
// Dogfood 6.6.36 round 1: `context list -p ""` and `ticket list -p ""` acted on
// the default project (exit 0) once one was set.
func TestExplicitEmptyProjectIsRefused3499(t *testing.T) {
	t.Setenv("STOMPY_PROJECT", "env-project")
	for _, v := range []string{"", "   "} {
		got, err := ResolveProjectFlag(v, true)
		if err == nil {
			t.Fatalf("ResolveProjectFlag(%q, given) = %q, want the empty-name refusal", v, got)
		}
		if !strings.Contains(err.Error(), "project name is empty") {
			t.Errorf("error = %q, want it to say the project name is empty", err)
		}
	}
}

func TestAnAbsentFlagStillFallsBack3499(t *testing.T) {
	t.Setenv("STOMPY_PROJECT", "env-project")
	if got, err := ResolveProjectFlag("", false); err != nil || got != "env-project" {
		t.Fatalf("ResolveProjectFlag(\"\", absent) = %q, %v; want env-project", got, err)
	}
	if got, err := ResolveProjectFlag("named", true); err != nil || got != "named" {
		t.Fatalf("ResolveProjectFlag(named, given) = %q, %v", got, err)
	}
}
