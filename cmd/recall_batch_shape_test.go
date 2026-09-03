package cmd

import (
	"encoding/json"
	"testing"
)

// STOMPY-1921: recall_batch results arrive as a topic-keyed object (6.6.x);
// the old list shape must keep parsing too.
func TestRecallBatchResponse_MapShape(t *testing.T) {
	body := `{"results":{"b_topic":{"topic":"b_topic","version":"1.0","content":"B"},"a_topic":{"topic":"a_topic","version":"1.1","content":"A"}},"found":["a_topic","b_topic"],"not_found":["zzz"]}`
	var r RecallBatchResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Results) != 3 || r.Results[0].Topic != "a_topic" || !r.Results[0].Found || r.Results[2].Topic != "zzz" || r.Results[2].Found {
		t.Fatalf("got %+v", r.Results)
	}
}

func TestRecallBatchResponse_ListShape(t *testing.T) {
	body := `{"results":[{"topic":"x","found":true,"content":"X"},{"topic":"y","found":false}]}`
	var r RecallBatchResponse
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Results) != 2 || r.Results[0].Content != "X" || r.Results[1].Found {
		t.Fatalf("got %+v", r.Results)
	}
}
