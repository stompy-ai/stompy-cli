package cmd

import (
	"fmt"
	"strings"

	"github.com/stompy-ai/stompy-cli/internal/api"
)

func nameParts(address string) (owner, name string) {
	owner, name, qualified := strings.Cut(strings.TrimPrefix(address, "@"), "/")
	if !qualified {
		return "", address
	}
	return owner, name
}

// A marker correction must describe the address actually observed, including
// its owner. Never turn a qualified binding into a bare-name guess.
func checkedRenamedAddress(requested string, response *api.ProjectResponse) (string, error) {
	owner, name := nameParts(requested)
	formerOwner, former := nameParts(response.RenamedFrom)
	currentOwner, current := nameParts(response.NowCalled)
	if response.EnrichmentWarning != "" || response.RenamedFrom == "" ||
		former != name || formerOwner != currentOwner ||
		(owner != "" && formerOwner != owner) || current != response.Name ||
		current == "" || strings.ContainsAny(current, "/\r\n\t ") ||
		strings.ContainsAny(currentOwner, "\r\n\t ") {
		return "", fmt.Errorf("project rename evidence is unverifiable; .stompy.yaml was not changed")
	}
	return response.NowCalled, nil
}
