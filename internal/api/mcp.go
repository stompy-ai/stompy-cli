package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// MCPClient wraps JSON-RPC 2.0 calls to the Stompy MCP endpoint.
type MCPClient struct {
	BaseURL    string // e.g., "https://api.stompy.ai/mcp"
	AuthToken  string
	UserAgent  string
	HTTPClient *http.Client
	Verbose    bool
	nextID     int64
}

// jsonRPCRequest is a JSON-RPC 2.0 request envelope.
type jsonRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response envelope.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// mcpToolResult is the MCP tools/call result envelope.
type mcpToolResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewMCPClient creates a new MCP client.
// mcpURL should be the full MCP endpoint (e.g., "https://api.stompy.ai/mcp").
func NewMCPClient(mcpURL, authToken, version string, verbose bool) *MCPClient {
	ua := "stompy-cli/dev"
	if version != "" && version != "dev" {
		ua = "stompy-cli/" + version
	}
	return &MCPClient{
		BaseURL:   strings.TrimRight(mcpURL, "/"),
		AuthToken: authToken,
		UserAgent: ua,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second, // MCP tools may take longer than REST
		},
		Verbose: verbose,
	}
}

// MCPBaseURL derives the MCP endpoint URL from a REST base URL.
// "https://api.stompy.ai/api/v1" → "https://api.stompy.ai/mcp"
func MCPBaseURL(restBaseURL string) string {
	u := strings.TrimRight(restBaseURL, "/")
	if idx := strings.Index(u, "/api/v1"); idx != -1 {
		u = u[:idx]
	}
	return u + "/mcp"
}

// CallTool sends a tools/call JSON-RPC request and returns the text content.
func (m *MCPClient) CallTool(toolName string, arguments map[string]any) (string, error) {
	id := atomic.AddInt64(&m.nextID, 1)

	reqBody := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling MCP request: %w", err)
	}

	if m.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] --> MCP POST %s (tool: %s)\n", m.BaseURL, toolName)
		preview := string(reqBytes)
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		fmt.Fprintf(os.Stderr, "[DEBUG]     Body: %s\n", preview)
	}

	req, err := http.NewRequest(http.MethodPost, m.BaseURL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("creating MCP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Streamable-HTTP MCP requires the client to accept both; offering only
	// application/json 406s (STOMPY-1462 cluster 1 — `context explore` and
	// `context dashboard` failed with "Client must accept both
	// application/json and text/event-stream").
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", m.UserAgent)
	if m.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.AuthToken)
	}

	start := time.Now()
	resp, err := m.HTTPClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return "", fmt.Errorf("executing MCP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading MCP response: %w", err)
	}

	if m.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] <-- %d %s (%s, %d bytes)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed, len(respBytes))
		preview := string(respBytes)
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		fmt.Fprintf(os.Stderr, "[DEBUG]     Body: %s\n", preview)
	}

	if resp.StatusCode != http.StatusOK {
		return "", &APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("MCP endpoint returned %d: %s", resp.StatusCode, string(respBytes)),
		}
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(respBytes, &rpcResp); err != nil {
		return "", fmt.Errorf("decoding MCP response: %w", err)
	}

	if rpcResp.Error != nil {
		return "", fmt.Errorf("MCP error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	var toolResult mcpToolResult
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil {
		return "", fmt.Errorf("decoding MCP tool result: %w", err)
	}

	if toolResult.IsError {
		if len(toolResult.Content) > 0 {
			return "", fmt.Errorf("MCP tool error: %s", toolResult.Content[0].Text)
		}
		return "", fmt.Errorf("MCP tool returned an error")
	}

	// Concatenate all text content items
	var texts []string
	for _, c := range toolResult.Content {
		if c.Type == "text" {
			texts = append(texts, c.Text)
		}
	}

	return strings.Join(texts, "\n"), nil
}

// CallToolTyped calls a tool and unmarshals the JSON text response into dest.
func (m *MCPClient) CallToolTyped(toolName string, arguments map[string]any, dest any) error {
	text, err := m.CallTool(toolName, arguments)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(text), dest); err != nil {
		return fmt.Errorf("decoding tool response as JSON: %w (raw: %.200s)", err, text)
	}
	return nil
}
