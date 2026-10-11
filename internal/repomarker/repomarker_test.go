package repomarker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const goodFile = `version: 1
environment: production
projects:
  - address: "@markus/stompy"
    role: primary
    default: true
    purpose: "the product"
  - address: "@alice/shared-docs"
    role: reference
    purpose: "read-only handbook"
`

func TestParseValid(t *testing.T) {
	m, err := Parse([]byte(goodFile))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if m.Environment != "production" || len(m.Projects) != 2 {
		t.Fatalf("unexpected marker: %+v", m)
	}
	if got := m.DefaultProject().Address; got != "@markus/stompy" {
		t.Errorf("default = %q", got)
	}
}

func TestParseValidationErrors(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"unknown_version", "version: 2\nprojects:\n  - address: a\n    role: primary\n    default: true\n", "version"},
		{"missing_version", "projects:\n  - address: a\n    role: primary\n    default: true\n", "version"},
		{"zero_defaults", "version: 1\nprojects:\n  - address: a\n    role: primary\n", "default"},
		{"multiple_defaults", "version: 1\nprojects:\n  - address: a\n    role: primary\n    default: true\n  - address: b\n    role: shared\n    default: true\n", "default"},
		{"bad_role", "version: 1\nprojects:\n  - address: a\n    role: owner\n    default: true\n", "role"},
		{"empty_address", "version: 1\nprojects:\n  - address: \"\"\n    role: primary\n    default: true\n", "address"},
		{"duplicate_address", "version: 1\nprojects:\n  - address: a\n    role: primary\n    default: true\n  - address: a\n    role: shared\n", "duplicate"},
		{"bad_environment", "version: 1\nenvironment: prod\nprojects:\n  - address: a\n    role: primary\n    default: true\n", "environment"},
		{"no_projects", "version: 1\nprojects: []\n", "project"},
		{"unknown_key", "version: 1\nproject:\n  - address: a\n", "project"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.body))
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), c.want) {
				t.Errorf("error %q does not teach about %q", err, c.want)
			}
		})
	}
}

func TestFindNearestWalksUpAndStopsAtHome(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "work", "repo")
	nested := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	// A marker in HOME itself must never apply to everything under it.
	if err := os.WriteFile(filepath.Join(home, FileName), []byte(goodFile), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := FindNearest(nested, home)
	if err != nil || got != "" {
		t.Fatalf("home marker must not be found: %q, %v", got, err)
	}
	if err := os.WriteFile(filepath.Join(repo, FileName), []byte(goodFile), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = FindNearest(nested, home)
	if err != nil || got != filepath.Join(repo, FileName) {
		t.Fatalf("FindNearest = %q, %v", got, err)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	m, err := Parse([]byte(goodFile))
	if err != nil {
		t.Fatal(err)
	}
	out, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	m2, err := Parse(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(m2.Projects) != 2 || m2.Projects[1].Role != "reference" || m2.Projects[0].Purpose != "the product" {
		t.Errorf("round trip lost data: %+v", m2)
	}
}

func TestRenderBlockNamesAddressesRolesEnvironment(t *testing.T) {
	m, _ := Parse([]byte(goodFile))
	b := RenderBlock(m)
	for _, want := range []string{
		BlockBegin, BlockEnd,
		`project="markus/stompy"`, `project="alice/shared-docs"`,
		"@markus/stompy", "reference", "read-only", "production", "default",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("block missing %q:\n%s", want, b)
		}
	}
	if strings.HasSuffix(b, "\n") {
		t.Error("block must not end with a newline (the file writer owns it)")
	}
}

func TestWireAddress(t *testing.T) {
	for in, want := range map[string]string{"@a/b": "a/b", "a/b": "a/b", "mine": "mine"} {
		if got := (Project{Address: in}).WireAddress(); got != want {
			t.Errorf("WireAddress(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestApplyBlockCreatesPreservesAndIsIdempotent(t *testing.T) {
	m, _ := Parse([]byte(goodFile))
	block := RenderBlock(m)

	created, err := ApplyBlock("", block)
	if err != nil || !strings.Contains(created, BlockBegin) || !strings.HasSuffix(created, "\n") {
		t.Fatalf("create: %q %v", created, err)
	}

	outside := "# My agents\n\nHand-written rules.\n"
	appended, err := ApplyBlock(outside, block)
	if err != nil || !strings.HasPrefix(appended, outside) {
		t.Fatalf("append must keep outside text verbatim: %q %v", appended, err)
	}

	// Stale block in the middle, text on both sides.
	stale := "top\n\n" + BlockBegin + "\nOLD\n" + BlockEnd + "\n\nbottom\n"
	replaced, err := ApplyBlock(stale, block)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(replaced, "OLD") || !strings.HasPrefix(replaced, "top\n\n") || !strings.HasSuffix(replaced, "\n\nbottom\n") {
		t.Errorf("replace damaged surrounding text: %q", replaced)
	}

	again, err := ApplyBlock(replaced, block)
	if err != nil || again != replaced {
		t.Errorf("second apply not byte-identical")
	}
	again2, _ := ApplyBlock(appended, block)
	if again2 != appended {
		t.Errorf("append case not idempotent")
	}
}

func TestApplyBlockRefusesBrokenMarkers(t *testing.T) {
	for name, body := range map[string]string{
		"only_begin": "x\n" + BlockBegin + "\ny\n",
		"only_end":   "x\n" + BlockEnd + "\n",
		"reversed":   BlockEnd + "\n" + BlockBegin + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ApplyBlock(body, "b"); err == nil {
				t.Error("expected refusal rather than clobbering the file")
			}
		})
	}
}
