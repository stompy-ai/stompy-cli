// Package contract carries the checked-in command -> REST route table that the
// CLI is held to (STOMPY-2728). The backend's surface_contract.json maps
// capabilities to command NAMES; nothing asserted which ROUTE a command calls.
package contract

import (
	_ "embed"
	"encoding/json"
)

//go:embed command_routes.json
var raw []byte

// Entry is one command and the single REST request it must issue.
type Entry struct {
	Capability   string   `json:"capability"`
	Command      string   `json:"command"`
	Args         []string `json:"args"`
	Method       string   `json:"method"`
	PathTemplate string   `json:"path_template"`
}

// Table is the parsed command_routes.json.
type Table struct {
	Source     string   `json:"source"`
	NotCovered []string `json:"not_covered"`
	Routes     []Entry  `json:"routes"`
}

// Load parses the embedded table.
func Load() (Table, error) {
	var t Table
	err := json.Unmarshal(raw, &t)
	return t, err
}
