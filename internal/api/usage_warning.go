package api

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// UsageWarning is the REST `usage_warning` sibling a 2xx write carries once
// the account crosses COMBINED_UNIT_WARN_PCT of its tier's cap (STOMPY-2201,
// STOMPY-2232). It rides beside the handler's own payload — never inside it —
// so Client.Do prints it to stderr and the decoded response struct (which has
// no such field) leaves stdout untouched.
//
// The sibling carries only the numbers (used, cap, pct, resets_at,
// upgrade_url): no tier name and no composed sentence, unlike the
// STOMPY-1870 cap-refusal shape, whose `detail` IS the server's sentence
// verbatim. Line() composes the CLI's own rendering from those numbers —
// it cannot reproduce the recap's tier name or next-tier cap, which the
// wire does not send.
type UsageWarning struct {
	Used       int     `json:"used"`
	Cap        int     `json:"cap"`
	Pct        int     `json:"pct"`
	ResetsAt   *string `json:"resets_at"`
	UpgradeURL *string `json:"upgrade_url"`
}

// usageWarningEnvelope decodes just the sibling out of a write response,
// ignoring everything else the handler's payload carries.
type usageWarningEnvelope struct {
	UsageWarning *UsageWarning `json:"usage_warning"`
}

// Line renders the sibling as the one line Client.Do prints to stderr.
// A windowed (monthly) tier names the reset date; a lifetime allowance
// (free) never has one. An upgrade_url gets an explicit action; its
// absence on a windowed tier means "contact us" (Power has no next rung).
func (w *UsageWarning) Line() string {
	used := formatThousands(w.Used)
	cap := formatThousands(w.Cap)

	var line string
	if w.ResetsAt != nil && *w.ResetsAt != "" {
		line = fmt.Sprintf("Usage: %s of %s units this month — resets %s.", used, cap, formatResetDate(*w.ResetsAt))
	} else {
		line = fmt.Sprintf("Usage: %s of %s active units (lifetime allowance).", used, cap)
	}

	switch {
	case w.UpgradeURL != nil && *w.UpgradeURL != "":
		line += " Upgrade: " + *w.UpgradeURL
	case w.ResetsAt == nil || *w.ResetsAt == "":
		line += " Archive contexts, tickets or documents you no longer need to stay under the allowance."
	default:
		line += " Contact us: support@stompy.ai"
	}
	return line
}

// printUsageWarning prints the sibling's line to STDERR when a 2xx body
// carries one (STOMPY-2232). It never touches stdout — the decoded response
// struct Post/Get hand back has no such field, so the caller's own stdout
// (including `-o json`) is unaffected — and it never fails the call: a body
// that isn't JSON, or has no sibling, is silently a no-op.
func printUsageWarning(body []byte) {
	var envelope usageWarningEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.UsageWarning == nil {
		return
	}
	fmt.Fprintln(os.Stderr, envelope.UsageWarning.Line())
}

// formatResetDate renders an ISO instant as "1 Oct"; the raw string when it
// doesn't parse (never worth failing the print over).
func formatResetDate(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%d %s", t.Day(), t.Format("Jan"))
}

// formatThousands renders an int with thousands separators ("1,200").
func formatThousands(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
