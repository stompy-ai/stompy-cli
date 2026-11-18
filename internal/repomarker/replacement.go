package repomarker

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReplacementLine is a pasteable correction bound to the validated source.
// An empty answer means use a value-only instruction: never guess a line for
// flow mappings, anchors, aliases, multiline values or an invalid result.
func (m *Marker) ReplacementLine(address, replacement string) string {
	var doc yaml.Node
	if yaml.Unmarshal(m.source, &doc) != nil || len(doc.Content) != 1 {
		return ""
	}
	root := doc.Content[0]
	var scalar *yaml.Node
	projectIndex := -1
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "projects" {
			continue
		}
		projects := root.Content[i+1]
		for index, item := range projects.Content {
			if item.Kind != yaml.MappingNode || item.Style != 0 || item.Anchor != "" {
				continue
			}
			for j := 0; j+1 < len(item.Content); j += 2 {
				value := item.Content[j+1]
				if item.Content[j].Value == "address" && value.Value == address {
					if scalar != nil {
						return ""
					}
					scalar, projectIndex = value, index
				}
			}
		}
	}
	if scalar == nil || scalar.Kind != yaml.ScalarNode || scalar.Anchor != "" {
		return ""
	}
	lines := strings.Split(string(m.source), "\n")
	if scalar.Line < 1 || scalar.Line > len(lines) || scalar.Column < 1 {
		return ""
	}
	original := lines[scalar.Line-1]
	line := []rune(strings.TrimSuffix(original, "\r"))
	start := scalar.Column - 1
	if start >= len(line) {
		return ""
	}
	prefix := string(line[:start])
	key := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(prefix), "-"))
	if key != "address:" {
		return ""
	}
	end := scalarEnd(line, start, scalar.Style)
	if end < 0 {
		return ""
	}
	target := replacement
	// @ is an optional display sigil. A plain YAML address cannot start
	// with it: preserve the original no-sigil spelling, never its owner.
	if scalar.Style == 0 && !strings.HasPrefix(address, "@") {
		oldOwner, _, qualified := strings.Cut(address, "/")
		newOwner, _, _ := strings.Cut(strings.TrimPrefix(replacement, "@"), "/")
		if qualified && oldOwner == newOwner {
			target = strings.TrimPrefix(replacement, "@")
		}
	}
	value := target
	switch scalar.Style {
	case yaml.SingleQuotedStyle:
		value = "'" + strings.ReplaceAll(target, "'", "''") + "'"
	case yaml.DoubleQuotedStyle:
		encoded, _ := json.Marshal(target)
		value = string(encoded)
	case 0:
	default:
		return ""
	}
	candidate := prefix + value + string(line[end:])
	lines[scalar.Line-1] = candidate
	if strings.HasSuffix(original, "\r") {
		lines[scalar.Line-1] += "\r"
	}
	checked, err := Parse([]byte(strings.Join(lines, "\n")))
	if err != nil || projectIndex >= len(checked.Projects) || checked.Projects[projectIndex].Address != target {
		return ""
	}
	return candidate
}

func scalarEnd(line []rune, start int, style yaml.Style) int {
	if style == 0 {
		for i := start; i < len(line); i++ {
			if line[i] == ' ' || line[i] == '\t' {
				return i
			}
		}
		return len(line)
	}
	if style != yaml.SingleQuotedStyle && style != yaml.DoubleQuotedStyle {
		return -1
	}
	quote := line[start]
	for i := start + 1; i < len(line); i++ {
		if style == yaml.DoubleQuotedStyle && line[i] == '\\' {
			i++
			continue
		}
		if line[i] == quote {
			if style == yaml.SingleQuotedStyle && i+1 < len(line) && line[i+1] == quote {
				i++
				continue
			}
			return i + 1
		}
	}
	return -1
}
