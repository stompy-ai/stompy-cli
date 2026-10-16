package api

import (
	"encoding/json"
	"testing"
)

// STOMPY-2860: decode the payload shapes the server really sends — created_at
// is epoch seconds (a number) and context_version is a string ("latest", "1.0").
// #35 shipped with ISO-string and int fixtures, so `ticket get`, `ticket link
// list` and `ticket link add` failed against a real server.
func TestLinkDecodesTheServersRealNumericAndStringTypes(t *testing.T) {
	body := `{"id":1,"title":"t","links":[
	  {"kind":"ticket","id":10,"source_id":1,"target_id":2,"link_type":"blocks","created_at":1792143289.62},
	  {"kind":"context","id":11,"ticket_id":1,"context_label":"rules","context_version":"latest","link_type":"relates_to","created_at":1792143300.5}]}`
	var tk TicketResponse
	if err := json.Unmarshal([]byte(body), &tk); err != nil {
		t.Fatalf("ticket get shape: %v", err)
	}
	if len(tk.Links) != 2 || tk.Links[1].ContextVersion != "latest" || tk.Links[1].CreatedAt == nil {
		t.Fatalf("unexpected decode: %+v", tk.Links)
	}
	var one TicketLinkResp
	if err := json.Unmarshal([]byte(`{"id":12,"source_id":1,"target_id":2,"link_type":"blocks","created_at":1792143289.62}`), &one); err != nil {
		t.Fatalf("link add shape: %v", err)
	}
}
