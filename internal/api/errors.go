package api

import "fmt"

type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Detail     string `json:"detail,omitempty"`
	// ErrorCode is the STOMPY-1870 sibling beside `detail` (e.g.
	// UNIT_CAP_REACHED, STOMPY-2202). It rides next to the sentence, never
	// nested inside it, so a script can branch on the code without parsing
	// prose while a human still reads the sentence.
	ErrorCode string `json:"error_code,omitempty"`
}

func (e *APIError) Error() string {
	prefix := fmt.Sprintf("API error %d", e.StatusCode)
	if e.ErrorCode != "" {
		prefix += fmt.Sprintf(" [%s]", e.ErrorCode)
	}
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s — %s", prefix, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", prefix, e.Message)
}
