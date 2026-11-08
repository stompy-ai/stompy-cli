package api

import (
	"fmt"
	"unicode/utf8"
)

// stripControls drops terminal controls from text the server hands back
// (STOMPY-3502): any project member writes titles, labels and bodies, and the
// CLI prints them raw, so an ESC sequence in one would clear the screen,
// disguise a link (OSC 8) or write the clipboard (OSC 52).
//
// Each control is replaced by a visible, inert mark rather than dropped, so a
// reader (often an agent) still sees that something was attempted, as the board
// reader's quoting has always shown it. It works on any byte stream, JSON or
// not (MCP tools answer TOON text):
//   - an unescaped \u00XX naming a control, or a \b \f \r short escape, becomes
//     the text \x1b (written \\x1b, so JSON stays valid). A backslash that is
//     itself escaped (\\u001b) is literal text and stays;
//   - a raw control byte becomes U+FFFD, valid anywhere. A CR directly before
//     an LF is a line ending, not a control, and stays.
//
// Controls are C0 except \n and \t, DEL, and C1 (U+0080-U+009F). \n and \t,
// and their escapes, stay. The CLI's own colours are added after decoding, so
// they are untouched.
func stripControls(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); {
		if b[i] == '\\' {
			j := i
			for j < len(b) && b[j] == '\\' {
				j++
			}
			run := j - i
			if run%2 == 1 {
				if n, v := controlEscape(b[j:]); n > 0 {
					out = append(out, b[i:j-1]...) // the escaped backslashes before it
					out = append(out, fmt.Sprintf(`\\x%02x`, v)...)
					i = j + n
					continue
				}
			}
			out = append(out, b[i:j]...)
			i = j
			continue
		}
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size <= 1 {
			out = append(out, b[i]) // not UTF-8: leave it to the decoder
			i++
			continue
		}
		if r == '\r' && i+1 < len(b) && b[i+1] == '\n' {
			out = append(out, '\r') // CRLF is a line ending (SSE frames, JSON whitespace)
		} else if isControl(r) {
			out = append(out, "\ufffd"...)
		} else {
			out = append(out, b[i:i+size]...)
		}
		i += size
	}
	return out
}

// isControl: a rune that steers a terminal rather than printing.
func isControl(r rune) bool {
	return (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f || (r >= 0x80 && r <= 0x9f)
}

// controlEscape returns how many bytes after a lone backslash make a control
// escape (`b`, `f`, `r`, or `uXXXX` naming a control) and the control's value,
// else 0.
func controlEscape(rest []byte) (int, int) {
	if len(rest) == 0 {
		return 0, 0
	}
	switch rest[0] {
	case 'b':
		return 1, '\b'
	case 'f':
		return 1, '\f'
	case 'r':
		return 1, '\r'
	case 'u':
		if len(rest) < 5 {
			return 0, 0
		}
		v := 0
		for _, c := range rest[1:5] {
			d := hexVal(c)
			if d < 0 {
				return 0, 0
			}
			v = v*16 + d
		}
		if isControl(rune(v)) {
			return 5, v
		}
	}
	return 0, 0
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
