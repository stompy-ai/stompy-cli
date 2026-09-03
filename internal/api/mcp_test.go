package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPBaseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://api.stompy.ai/api/v1", "https://api.stompy.ai/mcp"},
		{"https://api.stompy.ai/api/v1/", "https://api.stompy.ai/mcp"},
		{"https://api-staging.stompy.ai/api/v1", "https://api-staging.stompy.ai/mcp"},
		{"http://localhost:8000/api/v1", "http://localhost:8000/mcp"},
		{"http://localhost:8000", "http://localhost:8000/mcp"},
	}

	for _, tt := range tests {
		got := MCPBaseURL(tt.input)
		if got != tt.want {
			t.Errorf("MCPBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMCPClient_CallTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}
		// STOMPY-1462 cluster 1: streamable-HTTP MCP 406s a request whose
		// Accept header doesn't offer text/event-stream alongside
		// application/json.
		if got := r.Header.Get("Accept"); got != "application/json, text/event-stream" {
			t.Errorf("expected Accept to offer both application/json and text/event-stream, got %q", got)
		}

		// Decode request
		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}
		if req.JSONRPC != "2.0" {
			t.Errorf("expected jsonrpc 2.0, got %q", req.JSONRPC)
		}
		if req.Method != "tools/call" {
			t.Errorf("expected method tools/call, got %q", req.Method)
		}

		params := req.Params.(map[string]any)
		if params["name"] != "project_brief" {
			t.Errorf("expected tool name project_brief, got %q", params["name"])
		}

		// Respond with MCP result
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"summary":"Test project brief"}`},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewMCPClient(server.URL, "test-token", "0.2.0", false)
	text, err := client.CallTool("project_brief", map[string]any{"project": "test"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	expected := `{"summary":"Test project brief"}`
	if text != expected {
		t.Errorf("CallTool returned %q, want %q", text, expected)
	}
}

func TestMCPClient_CallToolTyped(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": `{"name":"test","context_count":5}`},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewMCPClient(server.URL, "token", "dev", false)

	var dest struct {
		Name         string `json:"name"`
		ContextCount int    `json:"context_count"`
	}
	if err := client.CallToolTyped("project_brief", map[string]any{"project": "test"}, &dest); err != nil {
		t.Fatalf("CallToolTyped failed: %v", err)
	}
	if dest.Name != "test" || dest.ContextCount != 5 {
		t.Errorf("unexpected result: %+v", dest)
	}
}

func TestMCPClient_CallToolError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "Tool error: project not found"},
				},
				"isError": true,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewMCPClient(server.URL, "token", "dev", false)
	_, err := client.CallTool("project_brief", map[string]any{"project": "nonexistent"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "MCP tool error: Tool error: project not found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMCPClient_RPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]any{
				"code":    -32601,
				"message": "Method not found",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewMCPClient(server.URL, "token", "dev", false)
	_, err := client.CallTool("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// STOMPY-1921: streamable-HTTP servers answer tools/call as an SSE stream
// when the client accepts text/event-stream (which STOMPY-1462 made us do).
// The JSON-RPC message rides inside a `data:` line; the client must read it.
func TestMCPClient_CallTool_SSEResponse(t *testing.T) {
	body := "event: message\r\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"hello from sse\"}],\"isError\":false}}\r\n\r\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); !strings.Contains(got, "text/event-stream") {
			t.Errorf("Accept header must include text/event-stream, got %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewMCPClient(srv.URL, "tok", "test", false)
	text, err := c.CallTool("project_brief", map[string]any{"project": "x"})
	if err != nil {
		t.Fatalf("CallTool over SSE: %v", err)
	}
	if text != "hello from sse" {
		t.Fatalf("got %q", text)
	}
}

func TestMCPClient_CallTool_SSEMultiFrame(t *testing.T) {
	// A ping/notification frame before the response frame must be skipped;
	// the frame whose id matches the request is the answer.
	body := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\nevent: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"second frame\"}],\"isError\":false}}\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := NewMCPClient(srv.URL, "tok", "test", false)
	text, err := c.CallTool("project_brief", map[string]any{})
	if err != nil {
		t.Fatalf("CallTool over multi-frame SSE: %v", err)
	}
	if text != "second frame" {
		t.Fatalf("got %q", text)
	}
}

func TestStripRecap(t *testing.T) {
	in := "📋 Stompy recap — p (stored data)\n\n⚑ Critical rules (1) — …\n\n---\n\nproject: p\nnarrative: hi"
	if got := stripRecap(in); got != "project: p\nnarrative: hi" {
		t.Fatalf("got %q", got)
	}
	if got := stripRecap("project: p"); got != "project: p" {
		t.Fatalf("non-recap text must pass through, got %q", got)
	}
}

func TestMCPClient_CallToolTyped_TOONTextIsTypedError(t *testing.T) {
	body := "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"📋 Stompy recap — p (stored data)\\n\\n---\\n\\nproject: p\\nnarrative: hi\"}],\"isError\":false}}"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := NewMCPClient(srv.URL, "tok", "test", false)
	var dest struct{ Project string }
	err := c.CallToolTyped("project_brief", map[string]any{}, &dest)
	var raw *NonJSONToolResult
	if !errors.As(err, &raw) {
		t.Fatalf("expected NonJSONToolResult, got %v", err)
	}
	if raw.Text != "project: p\nnarrative: hi" || raw.Tool != "project_brief" {
		t.Fatalf("got %+v", raw)
	}
}
