package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/banton/stompy-cli/internal/output"
	"github.com/spf13/cobra"
)

var ticketCmd = &cobra.Command{
	Use:   "ticket",
	Short: "Manage tickets",
}

var ticketCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new ticket",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			return fmt.Errorf("--title is required")
		}

		desc, _ := cmd.Flags().GetString("description")
		ticketType, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetString("priority")
		assignee, _ := cmd.Flags().GetString("assignee")
		tagsStr, _ := cmd.Flags().GetString("tags")

		req := api.TicketCreate{
			Title:    title,
			Type:     ticketType,
			Priority: priority,
		}
		if desc != "" {
			req.Description = &desc
		}
		if assignee != "" {
			req.Assignee = &assignee
		}
		if tagsStr != "" {
			req.Tags = strings.Split(tagsStr, ",")
			for i := range req.Tags {
				req.Tags[i] = strings.TrimSpace(req.Tags[i])
			}
		}

		resp, err := apiClient.CreateTicket(project, req)
		if err != nil {
			return err
		}

		if !isTableOutput() {
			f := getFormatter()
			fmt.Print(f.FormatRaw(resp))
			return nil
		}

		fmt.Printf("%s Ticket #%d created: %s\n", output.Success("✓"), resp.ID, resp.Title)
		return nil
	},
}

var ticketGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show ticket details",
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

		resp, err := apiClient.GetTicket(project, id)
		if err != nil {
			return err
		}

		f := getFormatter()
		tType, tStatus, tPriority := resp.Type, resp.Status, resp.Priority
		if isTableOutput() {
			tType = output.ColorType(tType)
			tStatus = output.ColorStatus(tStatus)
			tPriority = output.ColorPriority(tPriority)
		}
		fields := []output.KeyValue{
			{Key: "ID", Value: fmt.Sprintf("%d", resp.ID)},
			{Key: "Title", Value: resp.Title},
			{Key: "Type", Value: tType},
			{Key: "Status", Value: tStatus},
			{Key: "Priority", Value: tPriority},
		}
		if resp.Description != nil {
			fields = append(fields, output.KeyValue{Key: "Description", Value: *resp.Description})
		}
		if resp.Assignee != nil {
			fields = append(fields, output.KeyValue{Key: "Assignee", Value: *resp.Assignee})
		}
		if len(resp.Tags) > 0 {
			fields = append(fields, output.KeyValue{Key: "Tags", Value: strings.Join(resp.Tags, ", ")})
		}
		if resp.CreatedAt != nil {
			fields = append(fields, output.KeyValue{Key: "Created", Value: formatTimestamp(*resp.CreatedAt)})
		}
		if resp.UpdatedAt != nil {
			fields = append(fields, output.KeyValue{Key: "Updated", Value: formatTimestamp(*resp.UpdatedAt)})
		}

		fmt.Print(f.FormatSingle(fields))
		return nil
	},
}

var ticketUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a ticket",
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

		req := api.TicketUpdate{}
		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			req.Title = &v
		}
		if cmd.Flags().Changed("description") {
			v, _ := cmd.Flags().GetString("description")
			req.Description = &v
		}
		if cmd.Flags().Changed("priority") {
			v, _ := cmd.Flags().GetString("priority")
			req.Priority = &v
		}
		if cmd.Flags().Changed("assignee") {
			v, _ := cmd.Flags().GetString("assignee")
			req.Assignee = &v
		}
		if cmd.Flags().Changed("tags") {
			v, _ := cmd.Flags().GetString("tags")
			tags := strings.Split(v, ",")
			for i := range tags {
				tags[i] = strings.TrimSpace(tags[i])
			}
			req.Tags = tags
		}

		resp, err := apiClient.UpdateTicket(project, id, req)
		if err != nil {
			return err
		}

		if !isTableOutput() {
			f := getFormatter()
			fmt.Print(f.FormatRaw(resp))
			return nil
		}

		fmt.Printf("%s Ticket #%d updated: %s\n", output.Success("✓"), resp.ID, resp.Title)
		return nil
	},
}

var ticketMoveCmd = &cobra.Command{
	Use:   "move <id>",
	Short: "Transition a ticket to a new status",
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

		status, _ := cmd.Flags().GetString("status")
		if status == "" {
			return fmt.Errorf("--status is required")
		}

		resp, err := apiClient.TransitionTicket(project, id, status)
		if err != nil {
			return err
		}

		fmt.Printf("%s Ticket #%d moved to %s\n", output.Success("✓"), resp.ID, output.ColorStatus(resp.Status))
		return nil
	},
}

// closeStatusMap maps ticket type to terminal status.
var closeStatusMap = map[string]string{
	"task":     "done",
	"bug":      "resolved",
	"feature":  "shipped",
	"decision": "decided",
}

var ticketCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a ticket (infers terminal status from ticket type)",
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

		// Fetch ticket to determine type
		ticket, err := apiClient.GetTicket(project, id)
		if err != nil {
			return err
		}

		status, ok := closeStatusMap[ticket.Type]
		if !ok {
			status = "done" // fallback
		}

		resp, err := apiClient.TransitionTicket(project, id, status)
		if err != nil {
			return err
		}

		fmt.Printf("%s Ticket #%d closed (%s → %s)\n", output.Success("✓"), resp.ID, output.ColorStatus(ticket.Status), output.ColorStatus(resp.Status))
		return nil
	},
}

var ticketListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tickets",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		status, _ := cmd.Flags().GetString("status")
		ticketType, _ := cmd.Flags().GetString("type")
		priority, _ := cmd.Flags().GetString("priority")
		limit, _ := cmd.Flags().GetInt("limit")
		offset, _ := cmd.Flags().GetInt("offset")

		resp, err := apiClient.ListTickets(project, status, ticketType, priority, limit, offset)
		if err != nil {
			return err
		}

		f := getFormatter()
		colorize := isTableOutput()
		headers := []string{"ID", "TYPE", "STATUS", "PRIORITY", "TITLE", "ASSIGNEE"}
		var rows [][]string
		for _, t := range resp.Tickets {
			assignee := ""
			if t.Assignee != nil {
				assignee = *t.Assignee
			}
			tType, tStatus, tPriority := t.Type, t.Status, t.Priority
			if colorize {
				tType = output.ColorType(tType)
				tStatus = output.ColorStatus(tStatus)
				tPriority = output.ColorPriority(tPriority)
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", t.ID),
				tType,
				tStatus,
				tPriority,
				truncate(t.Title, 50),
				assignee,
			})
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nTotal: %d tickets\n", resp.Total)
		}
		return nil
	},
}

var ticketBoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Show ticket board",
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		view, _ := cmd.Flags().GetString("view")
		ticketType, _ := cmd.Flags().GetString("type")
		status, _ := cmd.Flags().GetString("status")

		resp, err := apiClient.GetBoard(project, view, ticketType, status)
		if err != nil {
			return err
		}

		if !isTableOutput() {
			f := getFormatter()
			fmt.Print(f.FormatRaw(resp))
			return nil
		}

		for _, col := range resp.Columns {
			header := output.ColorStatus(strings.ToUpper(col.Status))
			fmt.Printf("\n%s %s\n", header, output.Dim(fmt.Sprintf("(%d)", col.Count)))
			for _, t := range col.Tickets {
				assignee := ""
				if t.Assignee != nil {
					assignee = output.Dim(fmt.Sprintf(" @%s", *t.Assignee))
				}
				fmt.Printf("  #%-4d %s %s%s\n", t.ID, output.ColorPriority(fmt.Sprintf("[%s]", t.Priority)), truncate(t.Title, 50), assignee)
			}
		}
		fmt.Printf("\n%s\n", output.Dim(fmt.Sprintf("Total: %d tickets", resp.Total)))
		return nil
	},
}

var ticketSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search tickets",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}

		ticketType, _ := cmd.Flags().GetString("type")
		status, _ := cmd.Flags().GetString("status")
		limit, _ := cmd.Flags().GetInt("limit")

		resp, err := apiClient.SearchTickets(project, args[0], ticketType, status, limit)
		if err != nil {
			return err
		}

		f := getFormatter()
		colorize := isTableOutput()
		headers := []string{"ID", "TYPE", "STATUS", "PRIORITY", "TITLE"}
		var rows [][]string
		for _, t := range resp.Results {
			tType, tStatus, tPriority := t.Type, t.Status, t.Priority
			if colorize {
				tType = output.ColorType(tType)
				tStatus = output.ColorStatus(tStatus)
				tPriority = output.ColorPriority(tPriority)
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", t.ID),
				tType,
				tStatus,
				tPriority,
				truncate(t.Title, 50),
			})
		}

		fmt.Print(f.FormatTable(headers, rows))
		if isTableOutput() {
			fmt.Printf("\nFound: %d tickets\n", resp.Total)
		}
		return nil
	},
}

func init() {
	ticketCreateCmd.Flags().String("title", "", "Ticket title (required)")
	ticketCreateCmd.Flags().String("description", "", "Ticket description")
	ticketCreateCmd.Flags().String("type", "task", "Ticket type: task, bug, feature, decision")
	ticketCreateCmd.Flags().String("priority", "medium", "Priority: critical, high, medium, low")
	ticketCreateCmd.Flags().String("assignee", "", "Assignee")
	ticketCreateCmd.Flags().String("tags", "", "Comma-separated tags")

	ticketGetCmd.Flags()

	ticketUpdateCmd.Flags().String("title", "", "New title")
	ticketUpdateCmd.Flags().String("description", "", "New description")
	ticketUpdateCmd.Flags().String("priority", "", "New priority")
	ticketUpdateCmd.Flags().String("assignee", "", "New assignee")
	ticketUpdateCmd.Flags().String("tags", "", "New comma-separated tags")

	ticketMoveCmd.Flags().String("status", "", "Target status (required)")

	ticketListCmd.Flags().String("status", "", "Filter by status")
	ticketListCmd.Flags().String("type", "", "Filter by type")
	ticketListCmd.Flags().String("priority", "", "Filter by priority")
	ticketListCmd.Flags().Int("limit", 0, "Limit results")
	ticketListCmd.Flags().Int("offset", 0, "Offset for pagination")

	ticketBoardCmd.Flags().String("view", "summary", "Board view: kanban, summary")
	ticketBoardCmd.Flags().String("type", "", "Filter by type")
	ticketBoardCmd.Flags().String("status", "", "Filter by status")

	ticketSearchCmd.Flags().String("type", "", "Filter by type")
	ticketSearchCmd.Flags().String("status", "", "Filter by status")
	ticketSearchCmd.Flags().Int("limit", 0, "Limit results")

	ticketCmd.AddCommand(ticketCreateCmd)
	ticketCmd.AddCommand(ticketGetCmd)
	ticketCmd.AddCommand(ticketUpdateCmd)
	ticketCmd.AddCommand(ticketMoveCmd)
	ticketCmd.AddCommand(ticketCloseCmd)
	ticketCmd.AddCommand(ticketListCmd)
	ticketCmd.AddCommand(ticketBoardCmd)
	ticketCmd.AddCommand(ticketSearchCmd)
	rootCmd.AddCommand(ticketCmd)
}

func formatTimestamp(ts float64) string {
	sec := int64(ts)
	nsec := int64((ts - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).Local().Format("2006-01-02 15:04:05")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
