package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newTicketArchiveCmd(action string) *cobra.Command {
	use := action + " <id>"
	if action == "batch-archive" {
		use = action + " <id,id,...>"
	}
	command := &cobra.Command{Use: use, Short: "Archive parked or closed tickets, or restore without changing status", Args: cobra.ExactArgs(1)}
	if action == "batch-archive" {
		command.Flags().Bool("confirm", false, "Execute the batch; default is a read-only preview")
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}
		var ids []int
		for _, part := range strings.Split(args[0], ",") {
			id, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket IDs must be positive integers")
			}
			ids = append(ids, id)
		}
		if action == "batch-archive" {
			if len(ids) > 50 {
				return fmt.Errorf("provide at most 50 ticket IDs")
			}
			confirm, _ := cmd.Flags().GetBool("confirm")
			result, err := apiClient.BatchArchiveTickets(project, ids, confirm)
			if err != nil {
				return err
			}
			fmt.Print(getFormatter().FormatRaw(result))
			return nil
		}
		if len(ids) != 1 {
			return fmt.Errorf("use batch-archive for multiple tickets")
		}
		result, err := apiClient.ArchiveTicket(project, action, ids[0])
		if err != nil {
			return err
		}
		if result.Ticket == nil {
			return fmt.Errorf("archive response has no ticket")
		}
		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(result))
			return nil
		}
		fmt.Printf("#%d %s (status unchanged: %s)\n", result.Ticket.ID, result.Status, result.Ticket.Status)
		return nil
	}
	return command
}

func init() {
	for _, action := range []string{"archive", "unarchive", "batch-archive"} {
		ticketCmd.AddCommand(newTicketArchiveCmd(action))
	}
}
