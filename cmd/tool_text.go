package cmd

import (
	"fmt"
	"strings"

	"github.com/banton/stompy-cli/internal/api"
)

// printToolText prints a tool's server-rendered text answer (STOMPY-1921), or
// returns it as an error when the text IS a refusal: the server renders one as
// "success: false / error: CODE / message: ..." without the MCP isError flag,
// and printing it with exit 0 hid a revoked or unknown project from scripts
// (STOMPY-2927, STOMPY-3125). The error exits 1 like every REST-backed read.
func printToolText(raw *api.NonJSONToolResult) error {
	if code, message, ok := toolTextRefusal(raw.Text); ok {
		return fmt.Errorf("%s: %s", code, message)
	}
	fmt.Println(raw.Text)
	return nil
}

// toolTextRefusal reads the top-level "error:" and "message:" keys of a TOON
// refusal; a result has no top-level "error:" key.
func toolTextRefusal(text string) (code, message string, ok bool) {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "error: ") && code == "" {
			code = strings.TrimSpace(strings.TrimPrefix(line, "error: "))
		}
		if strings.HasPrefix(line, "message: ") && message == "" {
			message = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "message: ")), `"`)
		}
	}
	return code, message, code != ""
}
