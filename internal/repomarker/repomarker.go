// Package repomarker reads and writes the repo-level .stompy.yaml marker
// (STOMPY-2742): which Stompy project(s) belong to this repository, in which
// role, on which environment. Committed, no secrets.
package repomarker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	FileName   = ".stompy.yaml"
	BlockBegin = "<!-- stompy:begin -->"
	BlockEnd   = "<!-- stompy:end -->"

	RolePrimary   = "primary"
	RoleShared    = "shared"
	RoleReference = "reference"

	EnvProduction = "production"
	EnvStaging    = "staging"
)

// Project is one Stompy project this repository uses.
type Project struct {
	Address string `yaml:"address"`
	Role    string `yaml:"role"`
	Default bool   `yaml:"default,omitempty"`
	Purpose string `yaml:"purpose,omitempty"`
}

// Marker is the parsed, validated .stompy.yaml.
type Marker struct {
	Version     int       `yaml:"version"`
	Environment string    `yaml:"environment,omitempty"`
	Projects    []Project `yaml:"projects"`
}

// Parse decodes and validates a marker. Every error says how to fix it.
func Parse(data []byte) (*Marker, error) {
	var m Marker
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s is empty: expected `version: 1` and a `projects:` list (run `stompy init --project <address>`)", FileName)
		}
		return nil, fmt.Errorf("%s is not valid: %v (allowed keys: version, environment, projects[address, role, default, purpose])", FileName, err)
	}
	if err := m.validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Marker) validate() error {
	if m.Version != 1 {
		return fmt.Errorf("%s: unsupported version %d: this CLI understands `version: 1` only (upgrade the CLI with `stompy update`, or fix the file)", FileName, m.Version)
	}
	switch m.Environment {
	case "":
		m.Environment = EnvProduction
	case EnvProduction, EnvStaging:
	default:
		return fmt.Errorf("%s: environment %q is not valid: use `production` or `staging`", FileName, m.Environment)
	}
	if len(m.Projects) == 0 {
		return fmt.Errorf("%s: no projects listed: add at least one entry under `projects:` with an address, a role and `default: true`", FileName)
	}
	seen := map[string]bool{}
	defaults := 0
	for i, p := range m.Projects {
		addr := strings.TrimSpace(p.Address)
		if addr == "" {
			return fmt.Errorf("%s: projects[%d] has an empty address: use `@owner/project`, or a bare name for your own personal project", FileName, i)
		}
		if strings.ContainsAny(addr, " \t\n") || strings.Count(addr, "/") > 1 {
			return fmt.Errorf("%s: address %q is not valid: use `@owner/project` or a bare `project` name (no spaces, at most one `/`)", FileName, addr)
		}
		if seen[addr] {
			return fmt.Errorf("%s: duplicate address %q: list each project once", FileName, addr)
		}
		seen[addr] = true
		switch p.Role {
		case RolePrimary, RoleShared, RoleReference:
		default:
			return fmt.Errorf("%s: %q has role %q, which is not valid: use `primary`, `shared` or `reference` (reference is read-only)", FileName, addr, p.Role)
		}
		if p.Default {
			defaults++
		}
	}
	if defaults != 1 {
		return fmt.Errorf("%s: %d projects are marked `default: true`: exactly one must be, so an agent knows which project to use when the task names none", FileName, defaults)
	}
	return nil
}

// Load reads and validates the marker at path.
func Load(path string) (*Marker, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// FindNearest walks up from start looking for FileName and returns its path,
// or "" if none. It stops at the filesystem root and never reads home itself
// (or anything above it): a stray ~/.stompy.yaml must not silently apply to
// every project under the home directory.
func FindNearest(start, home string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	home = filepath.Clean(home)
	for {
		if home != "." && dir == home {
			return "", nil
		}
		candidate := filepath.Join(dir, FileName)
		if st, serr := os.Stat(candidate); serr == nil && !st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

// DefaultProject returns the single project marked default.
func (m *Marker) DefaultProject() Project {
	for _, p := range m.Projects {
		if p.Default {
			return p
		}
	}
	return Project{}
}

// Marshal renders the marker as the committed file.
func (m *Marker) Marshal() ([]byte, error) {
	body, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	header := "# Stompy project marker. Commit this file; it holds no secrets.\n" +
		"# The managed block in AGENTS.md / CLAUDE.md is generated from it by `stompy init`.\n"
	return append([]byte(header), body...), nil
}

// WireAddress is the form a Stompy tool call passes as project=: the
// canonical `@owner/project` without the display sigil (the resolver's origin
// grammar is `handle/project`; a leading @ would read as an email origin).
func (p Project) WireAddress() string {
	return strings.TrimPrefix(strings.TrimSpace(p.Address), "@")
}

// RenderBlock is the managed block, with no trailing newline. It is a pure
// function of the marker, so the block and the file cannot drift.
func RenderBlock(m *Marker) string {
	var b strings.Builder
	b.WriteString(BlockBegin + "\n")
	b.WriteString("## Stompy memory (generated from `.stompy.yaml` by `stompy init`; edit the file, not this block)\n\n")
	fmt.Fprintf(&b, "Environment: `%s`\n\n", m.Environment)
	b.WriteString("Pass exactly one of these as `project=` on every Stompy MCP tool call:\n\n")
	for _, p := range m.Projects {
		fmt.Fprintf(&b, "- `project=%q` (%s) - role `%s`", p.WireAddress(), p.Address, p.Role)
		if p.Default {
			b.WriteString(", **default**")
		}
		if p.Role == RoleReference {
			b.WriteString(", **read-only**")
		}
		if p.Purpose != "" {
			b.WriteString(" - " + p.Purpose)
		}
		b.WriteString("\n")
	}
	b.WriteString("\nRules:\n")
	fmt.Fprintf(&b, "- Use the default, `project=%q`, unless the task names another project listed above.\n", m.DefaultProject().WireAddress())
	b.WriteString("- Roles: `primary` is this repo's own project, `shared` is a team project you may write to, `reference` is READ-ONLY: recall and search it, never lock, update, delete or post to it.\n")
	b.WriteString("- Never guess or invent a project address that is not listed here.\n")
	b.WriteString(BlockEnd)
	return b.String()
}

// ApplyBlock returns existing with the managed block created (appended) or
// replaced in place. Text outside the markers is never touched. A file whose
// markers are unbalanced is refused rather than guessed at.
func ApplyBlock(existing, block string) (string, error) {
	begin := strings.Index(existing, BlockBegin)
	end := strings.Index(existing, BlockEnd)
	switch {
	case begin == -1 && end == -1:
		if existing == "" {
			return block + "\n", nil
		}
		sep := "\n"
		if !strings.HasSuffix(existing, "\n") {
			sep = "\n\n"
		}
		return existing + sep + block + "\n", nil
	case begin == -1 || end == -1 || end < begin:
		return "", fmt.Errorf("managed block markers are unbalanced (need one `%s` followed by one `%s`): fix or remove them by hand, then re-run", BlockBegin, BlockEnd)
	}
	return existing[:begin] + block + existing[end+len(BlockEnd):], nil
}
