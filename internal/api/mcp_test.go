package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
