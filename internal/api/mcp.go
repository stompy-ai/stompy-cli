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

	// STOMPY-1921: a streamable-HTTP server answers tools/call as an SSE
	// stream when the client accepts text/event-stream (which STOMPY-1462
	// made us do). The JSON-RPC message rides inside `data:` lines; a
	// notification frame may precede it. Plain JSON bodies still work.
	payload := respBytes
	if isSSE(resp.Header.Get("Content-Type"), respBytes) {
		payload, err = sseResponsePayload(respBytes, 1)
		if err != nil {
			return "", fmt.Errorf("decoding MCP SSE response: %w", err)
		}
	}

	var rpcResp jsonRPCResponse
	if err := json.Unmarshal(payload, &rpcResp); err != nil {
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

	return stripRecap(strings.Join(texts, "\n")), nil
}

// NonJSONToolResult is returned by CallToolTyped when the server answered
// with human-readable text (the 6.6.x TOON rendering) instead of JSON.
// Callers print Text as-is — it is the same rendering an MCP client sees.
type NonJSONToolResult struct {
	Tool string
	Text string
}

func (e *NonJSONToolResult) Error() string {
	return fmt.Sprintf("tool %s returned text, not JSON", e.Tool)
}

// stripRecap drops the session recap block the server prepends to the first
// tool result of a session ("📋 Stompy recap — …\n\n---\n\n<result>"), so
// typed decoding and raw printing both see only the tool's own output.
func stripRecap(text string) string {
	if !strings.HasPrefix(strings.TrimSpace(text), "📋 Stompy recap") {
		return text
	}
	if i := strings.Index(text, "\n---\n"); i >= 0 {
		return strings.TrimLeft(text[i+len("\n---\n"):], "\n")
	}
	return text
}

// CallToolTyped calls a tool and unmarshals the JSON text response into dest.
func (m *MCPClient) CallToolTyped(toolName string, arguments map[string]any, dest any) error {
	text, err := m.CallTool(toolName, arguments)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(text), dest); err != nil {
		if strings.TrimSpace(text) != "" && !strings.HasPrefix(strings.TrimSpace(text), "{") {
			// STOMPY-1921: since 6.6.13 tools render TOON text, not JSON.
			// Hand the text back so the command can print it verbatim.
			return &NonJSONToolResult{Tool: toolName, Text: text}
		}
		return fmt.Errorf("decoding tool response as JSON: %w (raw: %.200s)", err, text)
	}
	return nil
}

// isSSE reports whether the MCP response is a text/event-stream body.
func isSSE(contentType string, body []byte) bool {
	if strings.HasPrefix(strings.ToLower(contentType), "text/event-stream") {
		return true
	}
	trimmed := bytes.TrimLeft(body, " \r\n\t")
	return bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte("data:"))
}

// sseResponsePayload extracts the JSON-RPC response for `wantID` from an SSE
// body: frames are separated by blank lines, each frame's `data:` lines join
// to one JSON document. Frames without a matching id (notifications, pings)
// are skipped; when no frame carries the id, the last frame with a result or
// error is returned so a server that omits ids still parses.
func sseResponsePayload(body []byte, wantID int) ([]byte, error) {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	var fallback []byte
	for _, frame := range strings.Split(text, "\n\n") {
		var data []string
		for _, line := range strings.Split(frame, "\n") {
			if strings.HasPrefix(line, "data:") {
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		if len(data) == 0 {
			continue
		}
		doc := []byte(strings.Join(data, "\n"))
		var probe struct {
			ID     *json.Number    `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(doc, &probe); err != nil {
			continue
		}
		if probe.ID != nil {
			if id, err := probe.ID.Int64(); err == nil && int(id) == wantID {
				return doc, nil
			}
		}
		if len(probe.Result) > 0 || len(probe.Error) > 0 {
			fallback = doc
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, fmt.Errorf("no JSON-RPC response frame in SSE body (%d bytes)", len(body))
}
