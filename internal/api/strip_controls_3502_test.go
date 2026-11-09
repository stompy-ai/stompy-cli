package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// STOMPY-3502: text the server hands back (a ticket title, a context label, a
// tool's answer) is written by any project member and printed raw. A terminal
// control in it rewrites the screen, disguises a link (OSC 8) or writes the
// clipboard (OSC 52). Each control is replaced on the way in by a visible,
// inert mark (an escaped one by the text \x1b, a raw one by U+FFFD), so a
// reader still sees that something was attempted; \n and \t stay.

func decodeT(t *testing.T, b []byte) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(stripControls(b), &m); err != nil {
		t.Fatalf("stripped body is not JSON: %v\n%s", err, stripControls(b))
	}
	return m
}

func TestStripControlsMarksEscapedControls3502(t *testing.T) {
	m := decodeT(t, []byte(`{"t":"x\u001b]8;;http://evil\u001B\\y","c":"a\r\b\fz\u0007","k":"\u009b2J\u00e9"}`))
	if m["t"] != `x\x1b]8;;http://evil\x1b\y` {
		t.Errorf("t = %q", m["t"])
	}
	if m["c"] != `a\x0d\x08\x0cz\x07` {
		t.Errorf("c = %q", m["c"])
	}
	if m["k"] != `\x9b2Jé` {
		t.Errorf("k = %q", m["k"])
	}
}

func TestStripControlsKeepsTextThatOnlyLooksLikeAnEscape3502(t *testing.T) {
	m := decodeT(t, []byte(`{"q":"\\u001b and \\r","n":"a\nb\tc","s":"\"quoted\" \/ ok"}`))
	if m["q"] != `\u001b and \r` {
		t.Errorf("q = %q", m["q"])
	}
	if m["n"] != "a\nb\tc" {
		t.Errorf("n = %q", m["n"])
	}
	if m["s"] != `"quoted" / ok` {
		t.Errorf("s = %q", m["s"])
	}
}

func TestStripControlsMarksRawControls3502(t *testing.T) {
	got := string(stripControls([]byte("a\x1b[2Jb\xc2\x9bc\x7fd\r\ne\tf\rg")))
	if got != "a\ufffd[2Jb\ufffdc\ufffdd\r\ne\tf\ufffdg" {
		t.Errorf("got %q", got)
	}
}

func TestMCPToolTextIsStripped3502(t *testing.T) {
	for _, text := range []string{"title: \x1b]52;c;ZXZpbA==\x07Order", `{"name":"\u001b]52;c;ZXZpbA==\u0007Order"}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1,
				"result": map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}})
		}))
		client := NewMCPClient(server.URL, "token", "dev", false)
		got, err := client.CallTool("project_brief", map[string]any{"project": "p"})
		server.Close()
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(got, "\x1b\x07") || strings.Contains(got, `\u001b`) {
			t.Errorf("tool text kept a control: %q", got)
		}
	}
}

// Review of #51 (Kimi K3): the whole C1 range and DEL, escaped and raw; a lone
// C1 byte that is not valid UTF-8; and stripping twice changes nothing (the MCP
// path strips the body and then the decoded text).
func TestStripControlsCoversC1AndDEL3502(t *testing.T) {
	for v := 0x7f; v <= 0x9f; v++ {
		esc := fmt.Sprintf(`{"t":"a\u%04xb"}`, v)
		if got := decodeT(t, []byte(esc))["t"]; got != fmt.Sprintf(`a\x%02xb`, v) {
			t.Errorf("escaped U+%04X: %q", v, got)
		}
		raw := "a" + string(rune(v)) + "b"
		if got := string(stripControls([]byte(raw))); got != "a�b" {
			t.Errorf("raw U+%04X: %q", v, got)
		}
	}
	// Byte checks: ContainsAny would read "\x9b" as U+FFFD and match the mark.
	if got := string(stripControls([]byte("a\x9b2Jb\x9d8;;x"))); strings.IndexByte(got, 0x9b) >= 0 ||
		strings.IndexByte(got, 0x9d) >= 0 || got != "a\ufffd2Jb\ufffd8;;x" {
		t.Errorf("a lone C1 byte passed through: %q", got)
	}
}

func TestStripControlsIsIdempotent3502(t *testing.T) {
	in := []byte(`{"t":"x\u001b]52;c;e\u0007\\u001b \r\n\u009d","r":"` + "a\x1b\x9bb" + `"}`)
	once := stripControls(in)
	if twice := stripControls(once); string(twice) != string(once) {
		t.Errorf("strip twice:\n once %q\ntwice %q", once, twice)
	}
}
