package cmd

import (
	"testing"

	"github.com/banton/stompy-cli/internal/config"
	"github.com/spf13/viper"
)

func TestCommandNeedsAuth(t *testing.T) {
	cases := []struct {
		name             string
		cmdPath          string
		cmdName          string
		hasParent        bool
		isParentGrouping bool
		want             bool
	}{
		{
			name:      "whoami never needs auth resolution — it must run to report auth status itself",
			cmdPath:   "stompy whoami",
			cmdName:   "whoami",
			hasParent: true,
			want:      false,
		},
		{
			name:      "login never needs auth resolution",
			cmdPath:   "stompy login",
			cmdName:   "login",
			hasParent: true,
			want:      false,
		},
		{
			name:      "logout never needs auth resolution",
			cmdPath:   "stompy logout",
			cmdName:   "logout",
			hasParent: true,
			want:      false,
		},
		{
			name:      "version never needs auth resolution",
			cmdPath:   "stompy version",
			cmdName:   "version",
			hasParent: true,
			want:      false,
		},
		{
			name:      "update never needs auth resolution",
			cmdPath:   "stompy update",
			cmdName:   "update",
			hasParent: true,
			want:      false,
		},
		{
			name:      "a subcommand named update still needs auth (path, not name, decides)",
			cmdPath:   "stompy context update",
			cmdName:   "update",
			hasParent: true,
			want:      true,
		},
		{
			name:      "shell completion doesn't need auth",
			cmdPath:   "stompy completion bash",
			cmdName:   "bash",
			hasParent: true,
			want:      false,
		},
		{
			name:      "config subcommands don't need auth",
			cmdPath:   "stompy config show",
			cmdName:   "show",
			hasParent: true,
			want:      false,
		},
		{
			name:      "root command with no subcommand args doesn't need auth",
			cmdPath:   "stompy",
			cmdName:   "stompy",
			hasParent: false,
			want:      false,
		},
		{
			name:             "a parent grouping command with no args doesn't need auth",
			cmdPath:          "stompy context",
			cmdName:          "context",
			hasParent:        true,
			isParentGrouping: true,
			want:             false,
		},
		{
			name:      "an ordinary API-backed command needs auth",
			cmdPath:   "stompy project list",
			cmdName:   "list",
			hasParent: true,
			want:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commandNeedsAuth(tc.cmdPath, tc.cmdName, tc.hasParent, tc.isParentGrouping)
			if got != tc.want {
				t.Errorf("commandNeedsAuth(%q, %q, %v, %v) = %v, want %v",
					tc.cmdPath, tc.cmdName, tc.hasParent, tc.isParentGrouping, got, tc.want)
			}
		})
	}
}

func TestCurrentEnvironment(t *testing.T) {
	origFlag := flagUseStaging
	defer func() { flagUseStaging = origFlag }()

	flagUseStaging = false
	if got := currentEnvironment(); got != "production" {
		t.Errorf("currentEnvironment() with --use-staging=false = %q, want %q", got, "production")
	}

	flagUseStaging = true
	if got := currentEnvironment(); got != "staging" {
		t.Errorf("currentEnvironment() with --use-staging=true = %q, want %q", got, "staging")
	}
}

func TestResolveAPIURL(t *testing.T) {
	origAPIURL, origUseStaging := flagAPIURL, flagUseStaging
	defer func() { flagAPIURL, flagUseStaging = origAPIURL, origUseStaging }()

	// Isolate viper/HOME and load defaults, same as a real invocation would
	// via config.Load() in PersistentPreRunE — resolveAPIURL() reads
	// config.GetAPIURL()/GetStagingAPIURL(), which are backed by viper.
	viper.Reset()
	t.Setenv("HOME", t.TempDir())
	if err := config.Load(); err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	flagAPIURL = "https://override.example.com/api/v1"
	flagUseStaging = false
	if got := resolveAPIURL(); got != "https://override.example.com/api/v1" {
		t.Errorf("resolveAPIURL() with --api-url override = %q, want the override", got)
	}

	flagAPIURL = ""
	flagUseStaging = true
	if got := resolveAPIURL(); got != "https://api-staging.stompy.ai/api/v1" {
		t.Errorf("resolveAPIURL() with --use-staging = %q, want the staging URL", got)
	}

	flagAPIURL = ""
	flagUseStaging = false
	if got := resolveAPIURL(); got != "https://api.stompy.ai/api/v1" {
		t.Errorf("resolveAPIURL() with no flags = %q, want the production URL", got)
	}
}
