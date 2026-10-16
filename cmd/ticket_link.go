package cmd

import (
	"fmt"
	"strconv"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/spf13/cobra"
)

var ticketLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage ticket links",
}

var ticketLinkAddCmd = &cobra.Command{
	Use:   "add <ticket-id>",
	Short: "Add a link between tickets",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid ticket ID: %s", args[0])
		}

		target, _ := cmd.Flags().GetInt("target")
		if target == 0 {
			return fmt.Errorf("--target is required")
		}
		linkType, _ := cmd.Flags().GetString("type")
		if linkType == "" {
			return fmt.Errorf("--type is required (blocks, parent, related, duplicate)")
		}

		req := api.LinkCreate{
			TargetID: target,
			LinkType: linkType,
		}

		resp, err := apiClient.AddLink(project, id, req)
		if err != nil {
			return err
		}

		fmt.Printf("Link created: #%d -[%s]-> #%d\n", resp.SourceID, resp.LinkType, resp.TargetID)
		return nil
	},
}

var ticketLinkListCmd = &cobra.Command{
	Use:   "list <ticket-id>",
	Short: "List links for a ticket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid ticket ID: %s", args[0])
		}

		links, err := apiClient.ListLinks(project, id)
		if err != nil {
			return err
		}

		if len(links) == 0 {
			fmt.Println("No links found.")
			return nil
		}

		f := getFormatter()
		// json/yaml: pass the server's fields through (peer_*, direction,
		// relation, source_*/target_*) instead of flattening to columns.
		if of := getOutputFormat(); of == "json" || of == "yaml" {
			fmt.Print(f.FormatRaw(links))
			return nil
		}
		headers := []string{"LINK ID", "RELATION", "TICKET", "TITLE", "STATUS", "DIRECTION"}
		var rows [][]string
		for _, l := range links {
			rows = append(rows, linkRow(l))
		}

		fmt.Print(f.FormatTable(headers, rows))
		return nil
	},
}

var ticketLinkRemoveCmd = &cobra.Command{
	Use:   "remove <ticket-id> <link-id>",
	Short: "Remove a ticket link",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		ticketID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid ticket ID: %s", args[0])
		}
		linkID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid link ID: %s", args[1])
		}

		if err := apiClient.RemoveLink(project, ticketID, linkID); err != nil {
			return err
		}

		fmt.Printf("Link %d removed from ticket #%d\n", linkID, ticketID)
		return nil
	},
}

func init() {
	ticketLinkAddCmd.Flags().Int("target", 0, "Target ticket ID (required)")
	ticketLinkAddCmd.Flags().String("type", "", "Link type: blocks, parent, related, duplicate (required)")

	ticketLinkCmd.AddCommand(ticketLinkAddCmd)
	ticketLinkCmd.AddCommand(ticketLinkListCmd)
	ticketLinkCmd.AddCommand(ticketLinkRemoveCmd)
	ticketCmd.AddCommand(ticketLinkCmd)
}

// linkRow renders the counterpart end of a link. Backends >= 0.8.13 send
// peer_*; for older ones it falls back to the (ListLinks-normalized)
// target id and prints title/status only when they are known to describe it.
func linkRow(l api.TicketLinkResp) []string {
	if l.Kind == "context" {
		rel := l.Relation
		if rel == "" {
			rel = l.LinkType
		}
		return []string{fmt.Sprintf("%d", l.ID), rel, "-", fmt.Sprintf("%s@%s", l.ContextLabel, l.ContextVersion), "-", l.Direction}
	}
	if l.PeerID != 0 {
		rel := l.Relation
		if rel == "" {
			rel = l.LinkType
		}
		return []string{fmt.Sprintf("%d", l.ID), rel, fmt.Sprintf("%d", l.PeerID), l.PeerTitle, l.PeerStatus, l.Direction}
	}
	title, status := l.TargetTitle, l.TargetStatus
	if l.LegacyReversed {
		title, status = "", ""
	}
	return []string{fmt.Sprintf("%d", l.ID), l.LinkType, fmt.Sprintf("%d", l.TargetID), title, status, ""}
}
