package output

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format   string
		wantType string
	}{
		{"table", "*output.TableFormatter"},
		{"json", "*output.JSONFormatter"},
		{"yaml", "*output.YAMLFormatter"},
		{"", "*output.TableFormatter"},
		{"unknown", "*output.TableFormatter"},
	}
	for _, tt := range tests {
		f := NewFormatter(tt.format)
		got := fmt.Sprintf("%T", f)
		if got != tt.wantType {
			t.Errorf("NewFormatter(%q) type = %s, want %s", tt.format, got, tt.wantType)
		}
	}
}

// --- Table Formatter ---

func TestTableFormatter_FormatTable(t *testing.T) {
	f := &TableFormatter{}
	headers := []string{"Name", "Status"}
	rows := [][]string{
		{"project-a", "active"},
		{"project-b", "archived"},
	}

	result := f.FormatTable(headers, rows)

	if !strings.Contains(result, "project-a") {
		t.Error("FormatTable missing row data 'project-a'")
	}
	if !strings.Contains(result, "project-b") {
		t.Error("FormatTable missing row data 'project-b'")
	}
	// go-pretty uppercases headers by default
	upper := strings.ToUpper(result)
	if !strings.Contains(upper, "NAME") || !strings.Contains(upper, "STATUS") {
		t.Errorf("FormatTable missing headers, got:\n%s", result)
	}
}

func TestTableFormatter_FormatSingle(t *testing.T) {
	f := &TableFormatter{}
	fields := []KeyValue{
		{Key: "Name", Value: "my-project"},
		{Key: "Status", Value: "active"},
		{Key: "Created", Value: "2026-01-15"},
	}

	result := f.FormatSingle(fields)

	if !strings.Contains(result, "Name:") {
		t.Error("FormatSingle missing key 'Name:'")
	}
	if !strings.Contains(result, "my-project") {
		t.Error("FormatSingle missing value 'my-project'")
	}
}

func TestTableFormatter_FormatSingle_Empty(t *testing.T) {
	f := &TableFormatter{}
	result := f.FormatSingle(nil)
	if result != "" {
		t.Errorf("FormatSingle(nil) = %q, want empty", result)
	}
}

func TestTableFormatter_FormatRaw(t *testing.T) {
	f := &TableFormatter{}
	result := f.FormatRaw("hello world")
	if result != "hello world" {
		t.Errorf("FormatRaw() = %q, want %q", result, "hello world")
	}
}

// --- JSON Formatter ---

func TestJSONFormatter_FormatTable(t *testing.T) {
	f := &JSONFormatter{}
	headers := []string{"Name", "Status"}
	rows := [][]string{
		{"project-a", "active"},
		{"project-b", "archived"},
	}

	result := f.FormatTable(headers, rows)

	var parsed []map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatTable JSON parse error: %v", err)
	}
	if len(parsed) != 2 {
		t.Fatalf("FormatTable got %d items, want 2", len(parsed))
	}
	if parsed[0]["Name"] != "project-a" {
		t.Errorf("parsed[0][Name] = %q, want %q", parsed[0]["Name"], "project-a")
	}
	if parsed[1]["Status"] != "archived" {
		t.Errorf("parsed[1][Status] = %q, want %q", parsed[1]["Status"], "archived")
	}
}

func TestJSONFormatter_FormatSingle(t *testing.T) {
	f := &JSONFormatter{}
	fields := []KeyValue{
		{Key: "Name", Value: "my-project"},
		{Key: "Status", Value: "active"},
	}

	result := f.FormatSingle(fields)

	var parsed map[string]string
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatSingle JSON parse error: %v", err)
	}
	if parsed["Name"] != "my-project" {
		t.Errorf("parsed[Name] = %q, want %q", parsed["Name"], "my-project")
	}
}

func TestJSONFormatter_FormatRaw(t *testing.T) {
	f := &JSONFormatter{}
	data := map[string]int{"count": 42}

	result := f.FormatRaw(data)

	var parsed map[string]int
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("FormatRaw JSON parse error: %v", err)
	}
	if parsed["count"] != 42 {
		t.Errorf("parsed[count] = %d, want 42", parsed["count"])
	}
}

// --- YAML Formatter ---

func TestYAMLFormatter_FormatTable(t *testing.T) {
	f := &YAMLFormatter{}
	headers := []string{"Name", "Status"}
	rows := [][]string{
		{"project-a", "active"},
	}

	result := f.FormatTable(headers, rows)

	if !strings.Contains(result, "Name: project-a") {
		t.Errorf("FormatTable YAML missing 'Name: project-a', got:\n%s", result)
	}
	if !strings.Contains(result, "Status: active") {
		t.Errorf("FormatTable YAML missing 'Status: active', got:\n%s", result)
	}
}

func TestYAMLFormatter_FormatSingle(t *testing.T) {
	f := &YAMLFormatter{}
	fields := []KeyValue{
		{Key: "Name", Value: "my-project"},
	}

	result := f.FormatSingle(fields)

	if !strings.Contains(result, "Name: my-project") {
		t.Errorf("FormatSingle YAML missing 'Name: my-project', got:\n%s", result)
	}
}

func TestYAMLFormatter_FormatRaw(t *testing.T) {
	f := &YAMLFormatter{}
	result := f.FormatRaw("raw-value")

	if !strings.Contains(result, "raw-value") {
		t.Errorf("FormatRaw YAML missing 'raw-value', got:\n%s", result)
	}
}

// STOMPY-2863: a single-object view (context recall, ticket get, ...) must
// never cut a value; truncation belongs to list tables only.
func TestTableFormatter_FormatSingle_NeverTruncates(t *testing.T) {
	f := &TableFormatter{}
	content := "tinyqueue releases MUST follow semver. MUST NOT publish to PyPI from a laptop; " +
		"only the GitHub Actions release workflow publishes. " + strings.Repeat("Tags and changelog are cut by the release workflow. ", 600)
	result := f.FormatSingle([]KeyValue{{Key: "Topic", Value: "release_policy"}, {Key: "Content", Value: content}})
	if !strings.Contains(result, content) {
		t.Errorf("FormatSingle cut the value: got %d bytes for %d-byte content", len(result), len(content))
	}
}

// STOMPY-2868: the full UUID must sit on one line so it can be copied.
func TestTableFormatter_FormatTable_IDColumnNotWrapped(t *testing.T) {
	f := &TableFormatter{}
	id := "3f2b8c1e-9d47-4a6b-8e15-0c7d2a9b4f61"
	result := f.FormatTable(
		[]string{"ID", "ITEM A", "ITEM B", "TYPE", "CONFIDENCE", "STATUS"},
		[][]string{{id, "context #101", "context #202", "direct_contradiction", "0.92", "pending"}},
	)
	if !strings.Contains(result, id) {
		t.Errorf("ID column wrapped or cut the UUID:\n%s", result)
	}
}
