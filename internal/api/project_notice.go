package api

import (
	"encoding/json"
	"fmt"
	"os"
)

// Read only the successful response's siblings, never authored content. Quote
// server strings so a notice remains one terminal line and stdout stays data.
func printProjectNotice(body []byte) {
	var envelope struct {
		NowCalled         string          `json:"now_called"`
		Success           *bool           `json:"success"`
		Error             json.RawMessage `json:"error"`
		EnrichmentWarning string          `json:"enrichment_warning"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.NowCalled == "" ||
		(envelope.Success != nil && !*envelope.Success) ||
		(len(envelope.Error) > 0 && string(envelope.Error) != "null") || envelope.EnrichmentWarning != "" {
		return
	}
	fmt.Fprintf(os.Stderr, "Project is now called %q.\n", envelope.NowCalled)
}
