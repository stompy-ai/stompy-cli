package cmd

import (
	"fmt"

	"github.com/stompy-ai/stompy-cli/internal/api"
)

// ticketLinkLines renders a ticket's links one per line (STOMPY-3500): a ticket
// link as `<relation> #<peer> <title> (<status>)`, a context link as
// `<link type> context <label>@<version>`. Empty when the ticket has none.
func ticketLinkLines(t api.TicketResponse) []string {
	var lines []string
	for _, l := range t.Links {
		r := linkRow(l) // [id, relation, ticket, title, status, direction]
		if l.Kind == "context" {
			lines = append(lines, fmt.Sprintf("%s context %s", r[1], r[3]))
			continue
		}
		line := fmt.Sprintf("%s #%s %s", r[1], r[2], r[3])
		if r[4] != "" {
			line += fmt.Sprintf(" (%s)", r[4])
		}
		lines = append(lines, line)
	}
	for _, c := range t.ContextLinks {
		version := c.ContextVersion
		if version == "" {
			version = "latest"
		}
		lines = append(lines, fmt.Sprintf("%s context %s@%s", c.LinkType, c.ContextLabel, version))
	}
	return lines
}

// printTicketLinks prints the lines under a mutation's one-line summary.
func printTicketLinks(t api.TicketResponse) {
	for _, line := range ticketLinkLines(t) {
		fmt.Printf("  link: %s\n", line)
	}
}
