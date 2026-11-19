package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/output"
	"gopkg.in/yaml.v3"
)

// Keep scan outcome separate from output format: an incomplete JSON result
// is useful data, but must still fail the command (STOMPY-3101/3380).
func printConflictDetection(resp *api.ConflictDetectResponse, queued *api.ConflictDetectQueuedResponse) error {
	incomplete := resp != nil && resp.Status == "incomplete"
	if !isTableOutput() {
		raw := queuedRaw(resp, queued)
		if getOutputFormat() == "json" {
			// Pass through the full response without rounding IDs or dropping
			// unknown fields; the API client's existing control filter applies.
			fmt.Println(string(raw))
		} else {
			var value any
			if err := yaml.Unmarshal(raw, &value); err != nil {
				return fmt.Errorf("decoding scan output: %w", err)
			}
			fmt.Print(getFormatter().FormatRaw(value))
		}
	} else if queued != nil {
		// STOMPY-1389: a queued scan has no result counts yet.
		fmt.Printf("%s %s (job %s)\n", output.Success("⧗"), queued.Message, queued.JobID)
	} else {
		mark := output.Success("✓")
		if incomplete {
			mark = "Incomplete scan:"
			reason := resp.Reason
			if reason == "" {
				reason = "unknown"
			}
			fmt.Printf("Status: incomplete; reason: %s; pairs examined: %s of %s\n",
				reason, scanCount(resp.PairsExamined), scanCount(resp.PairsTotal))
			if resp.Hint != "" {
				fmt.Printf("Hint: %s\n", resp.Hint)
			}
			printScanPairs("Unjudged", resp.UnjudgedPairs, resp.UnjudgedIDs)
			printScanPairs("Likely unconfirmed", resp.LikelyUnconfirmedPairs, resp.LikelyUnconfirmedIDs)
		}
		fmt.Printf("%s Found %d conflicts (%d auto-resolved, %d pending) in %.0fms\n",
			mark, resp.ConflictsFound, resp.AutoResolved, resp.Pending, resp.ProcessingTimeMs)
	}
	if incomplete {
		return &exitError{1, ""}
	}
	return nil
}

func queuedRaw(resp *api.ConflictDetectResponse, queued *api.ConflictDetectQueuedResponse) []byte {
	if queued != nil {
		return queued.Raw
	}
	return resp.Raw
}

func scanCount(count *int) string {
	if count == nil {
		return "unknown"
	}
	return strconv.Itoa(*count)
}

func printScanPairs(label string, count int, pairs [][]int) {
	if count == 0 && len(pairs) == 0 {
		return
	}
	ids := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts := make([]string, 0, len(pair))
		for _, id := range pair {
			parts = append(parts, strconv.Itoa(id))
		}
		ids = append(ids, strings.Join(parts, ","))
	}
	fmt.Printf("%s pairs: %d; IDs: %s\n", label, count, strings.Join(ids, "; "))
}
