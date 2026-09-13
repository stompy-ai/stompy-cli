package cmd

import (
	"fmt"
	"strconv"

	"github.com/banton/stompy-cli/internal/api"
	"github.com/spf13/cobra"
)

func newTicketLeaseCmd(action string) *cobra.Command {
	use := action + " <id>"
	args := cobra.ExactArgs(1)
	if action == "claim-next" {
		use = action
		args = cobra.NoArgs
	}
	command := &cobra.Command{Use: use, Short: "Atomically claim, renew or release a ticket lease", Args: args}
	command.Flags().String("agent-label", "", "Caller-supplied agent label; account comes from authentication")
	if action != "release" {
		command.Flags().Int("ttl-minutes", 60, "Lease length, 1..480 minutes; re-read updated_at after claiming")
	}
	if action == "claim-next" {
		command.Flags().String("assignee", "", "Required lane to claim from")
	}
	command.RunE = func(cmd *cobra.Command, args []string) error {
		project, err := getProject()
		if err != nil {
			return err
		}
		id := 0
		if action != "claim-next" {
			id, err = strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("ticket ID must be a positive integer")
			}
		}
		label, _ := cmd.Flags().GetString("agent-label")
		request := api.TicketLeaseRequest{AgentLabel: label}
		if action != "release" {
			request.TTLMinutes, _ = cmd.Flags().GetInt("ttl-minutes")
			if request.TTLMinutes < 1 || request.TTLMinutes > 480 {
				return fmt.Errorf("--ttl-minutes must be between 1 and 480")
			}
		}
		if action == "claim-next" {
			request.Assignee, _ = cmd.Flags().GetString("assignee")
			if request.Assignee == "" {
				return fmt.Errorf("--assignee is required")
			}
		}
		result, err := apiClient.LeaseTicket(project, action, id, request)
		if err != nil {
			return err
		}
		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(result))
			return nil
		}
		if result.Status == "empty" {
			fmt.Println("No unclaimed eligible tickets.")
			return nil
		}
		if result.Ticket == nil {
			return fmt.Errorf("lease response has no ticket")
		}
		fmt.Printf("#%d %q %s\n", result.Ticket.ID, result.Status, leaseDisplay(*result.Ticket))
		return nil
	}
	return command
}

func leaseDisplay(ticket api.TicketResponse) string {
	if ticket.ClaimedBy == nil || ticket.ClaimedUntil == nil {
		return ""
	}
	return fmt.Sprintf("account:%d agent:%q until %s", ticket.ClaimedBy.AccountID, ticket.ClaimedBy.AgentLabel, formatTimestamp(*ticket.ClaimedUntil))
}

func init() {
	for _, action := range []string{"claim", "release", "claim-next"} {
		ticketCmd.AddCommand(newTicketLeaseCmd(action))
	}
}
