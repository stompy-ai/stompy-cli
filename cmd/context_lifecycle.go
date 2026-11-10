package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

// lifecycleStates is the closed vocabulary of the server's lifecycle door.
var lifecycleStates = []string{"active", "parked", "delivered", "archived"}

// STOMPY-2722: the CLI door of set_lifecycle. Nothing is deleted; every
// transition is changelogged and reversible with --state active.
var contextLifecycleCmd = &cobra.Command{
	Use:   "lifecycle <topic>",
	Short: "Park, deliver, archive or re-activate a context",
	Long: `Transition a context's lifecycle state. Nothing is deleted: every
transition is changelogged and reversible with --state active.

  active     current working memory (the only state push surfaces show)
  parked     deliberately dormant, with a review clock (default 60 days)
  delivered  a PR/doc/email now carries this content; memory keeps the pointer
  archived   historical, still recallable by handle

Re-park with explicit days and --renewal-key to renew the review clock.
Reuse the key for retries; use a new key for a new human confirmation.

Accepts deeplink syntax:

  stompy context lifecycle project/topic --state archived
  stompy context lifecycle topic -p project --state parked --review-after-days 30 --note "waiting on infra"
  stompy context lifecycle topic -p project --state active     # undo an archive`,
	Args: exactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		state, _ := cmd.Flags().GetString("state")
		if state == "" {
			return fmt.Errorf("--state is required (one of: %s)", strings.Join(lifecycleStates, ", "))
		}
		if !isLifecycleState(state) {
			return fmt.Errorf("invalid --state %q (one of: %s)", state, strings.Join(lifecycleStates, ", "))
		}
		note, _ := cmd.Flags().GetString("note")
		req := api.ContextLifecycleRequest{State: state, Note: note}
		req.RenewalKey, _ = cmd.Flags().GetString("renewal-key")
		if cmd.Flags().Changed("renewal-key") && (strings.TrimSpace(req.RenewalKey) == "" || state != "parked" || !cmd.Flags().Changed("review-after-days")) {
			return fmt.Errorf("--renewal-key requires nonblank text, --state parked and --review-after-days")
		}
		if cmd.Flags().Changed("review-after-days") {
			days, _ := cmd.Flags().GetInt("review-after-days")
			if days < 0 {
				return fmt.Errorf("--review-after-days must be 0 or more, got %d", days)
			}
			req.ReviewAfterDays = &days
		}

		project, topic, _, err := resolveTopicRef(args[0])
		if err != nil {
			return err
		}

		resp, err := apiClient.SetContextLifecycle(project, topic, req)
		if err != nil {
			return err
		}

		if !isTableOutput() {
			fmt.Print(getFormatter().FormatRaw(resp))
			return nil
		}
		fmt.Println(lifecycleSummary(resp))
		return nil
	},
}

func isLifecycleState(s string) bool {
	for _, v := range lifecycleStates {
		if s == v {
			return true
		}
	}
	return false
}

// lifecycleSummary is the human line: topic, previous -> new state, and the
// review date for a parked context.
func lifecycleSummary(r *api.ContextLifecycleResponse) string {
	if !r.Changed {
		note := r.Note
		if note == "" {
			note = "no change"
		}
		line := fmt.Sprintf("%s %s is already %s (%s)", output.Dim("="), output.Teal(r.Topic), r.Lifecycle, note)
		if r.ReviewAfter != "" {
			line += fmt.Sprintf(" (receipt review by %s)", r.ReviewAfter)
		}
		return line
	}
	line := fmt.Sprintf("%s %s: %s → %s", output.Success("✓"), output.Teal(r.Topic), r.Previous, r.Lifecycle)
	if r.ReviewAfter != "" {
		line += fmt.Sprintf(" (review by %s)", r.ReviewAfter)
	}
	return line
}

func init() {
	contextLifecycleCmd.Flags().String("state", "", "Target state: active, parked, delivered, archived (required)")
	contextLifecycleCmd.Flags().Int("review-after-days", 0, "Parked only: days until the review (server default 60, max 3650)")
	contextLifecycleCmd.Flags().String("note", "", "One-line changelog note (shown after the topic)")
	contextLifecycleCmd.Flags().String("renewal-key", "", "Explicit re-park confirmation key; reuse for retries, new key for a new renewal")
	contextCmd.AddCommand(contextLifecycleCmd)
}
