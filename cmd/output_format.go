package cmd

import (
	"fmt"
	"strings"

	"github.com/stompy-ai/stompy-cli/internal/config"
)

func rawOutputFormat() string {
	if flagOutput != "" {
		return flagOutput
	}
	return config.GetOutputFormat()
}

// Both presentation decisions use the same normalized flag/config value.
func getOutputFormat() string {
	return strings.ToLower(strings.TrimSpace(rawOutputFormat()))
}

func validateOutputFormat() error {
	switch getOutputFormat() {
	case "table", "json", "yaml":
		return nil
	default:
		return &exitError{2, fmt.Sprintf("unknown output format %q: use table, json or yaml", rawOutputFormat())}
	}
}
