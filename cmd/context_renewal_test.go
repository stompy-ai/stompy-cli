package cmd

import (
	"strings"
	"testing"
)

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
	_, _, call, err := runLifecycle(t,
		jsonReply(200, `{"topic":"waiting","lifecycle":"parked","changed":true}`),
		"-p", "proj", "-o", "json", "context", "lifecycle", "waiting",
		"--state", "parked", "--review-after-days", "0", "--renewal-key", "human-confirmation-2")
	if err != nil || call.body["renewal_key"] != "human-confirmation-2" {
		t.Fatalf("new confirmation lost its marshalled intent: %#v, %v", call.body, err)
	}
}

func TestContextLifecycle_RenewalKeyRequiresParkedAndExplicitDays(t *testing.T) {
	for _, args := range [][]string{
		{"--state", "active", "--review-after-days", "60", "--renewal-key", "confirmation"},
		{"--state", "parked", "--renewal-key", "confirmation"},
		{"--state", "parked", "--review-after-days", "60", "--renewal-key", " "},
	} {
		_, _, call, err := runLifecycle(t, jsonReply(200, `{}`),
			append([]string{"-p", "proj", "-o", "json", "context", "lifecycle", "waiting"}, args...)...)
		if err == nil || !strings.Contains(err.Error(), "--renewal-key") {
			t.Fatalf("invalid renewal accepted: %v, %v", args, err)
		}
		if call.method != "" {
			t.Fatalf("invalid intent reached REST: %#v", call)
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
