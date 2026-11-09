package cmd

import "testing"

func TestContextLifecycle_RenewalKeySurvivesRetries(t *testing.T) {
	if contextLifecycleCmd.Flags().Lookup("renewal-key") == nil {
		t.Fatal("lifecycle cannot carry a renewal intent: --renewal-key is absent")
	}
	for i := 0; i < 2; i++ {
		_, _, call, err := runLifecycle(t,
			jsonReply(200, `{"topic":"waiting","lifecycle":"parked","changed":false,"note":"renewal already applied"}`),
			"-p", "proj", "-o", "json", "context", "lifecycle", "waiting",
			"--state", "parked", "--review-after-days", "0", "--renewal-key", "human-confirmation-1")
		if err != nil {
			t.Fatal(err)
		}
		if call.body["renewal_key"] != "human-confirmation-1" || call.body["review_after_days"] != float64(0) {
			t.Fatalf("retry %d lost its intent: %#v", i, call.body)
		}
	}
}

func TestContextLifecycle_WithoutRenewalKeyKeepsLegacyRequest(t *testing.T) {
	_, _, call, err := runLifecycle(t,
		jsonReply(200, `{"topic":"waiting","lifecycle":"active","changed":false}`),
		"-p", "proj", "-o", "json", "context", "lifecycle", "waiting", "--state", "active")
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := call.body["renewal_key"]; exists {
		t.Fatalf("invented a renewal intent: %#v", call.body)
	}
}
