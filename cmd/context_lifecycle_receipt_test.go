package cmd

import (
	"strings"
	"testing"
)

func TestContextLifecycle_ReplayReceiptDateIsVisible(t *testing.T) {
	for _, state := range []string{"parked", "active"} {
		t.Run(state, func(t *testing.T) {
			note := "renewal already applied"
			if state == "active" {
				note += "; superseded by current active lifecycle"
			}
			reply := jsonReply(200, `{"topic":"waiting","lifecycle":"`+state+`","changed":false,"review_after":"2027-01-08","note":"`+note+`"}`)
			out, _, _, err := runLifecycle(t, reply, "-p", "proj", "context", "lifecycle", "waiting", "--state", "parked", "--review-after-days", "60", "--renewal-key", "confirmation-1")
			if err != nil {
				t.Fatal(err)
			}
			for _, value := range []string{"waiting", state, note, "receipt review by 2027-01-08"} {
				if !strings.Contains(out, value) {
					t.Errorf("output %q lacks %q", out, value)
				}
			}
		})
	}
}
