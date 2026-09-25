package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	maxRetries   = 2
	retryBaseDelay = 1 * time.Second
)

type Client struct {
	BaseURL    string
	AuthToken  string
	UserAgent  string
	Version    string // CLI version (e.g., "0.2.0" or "dev")
	HTTPClient *http.Client
	Verbose    bool
	NoCache    bool // When true, sends Cache-Control: no-cache (consumed after each Do call)

	// Server version info (populated from response headers)
	APIVersion   string // X-Stompy-API-Version
	compatWarned bool   // only warn once per invocation
}

func NewClient(baseURL, authToken, version string, verbose bool) *Client {
	ua := "stompy-cli/dev"
	if version != "" && version != "dev" {
		ua = "stompy-cli/" + version
	}
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		AuthToken: authToken,
		UserAgent: ua,
		Version:   version,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Verbose: verbose,
	}
}

func (c *Client) Do(method, path string, body any, params url.Values) ([]byte, int, error) {
	u := c.BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	var reqBytes []byte
	if body != nil {
		var err error
		reqBytes, err = json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling request body: %w", err)
		}
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] --> %s %s\n", method, u)
		if len(reqBytes) > 0 {
			preview := string(reqBytes)
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			fmt.Fprintf(os.Stderr, "[DEBUG]     Body: %s\n", preview)
		}
	}

	retries := 0
	if isIdempotent(method) {
		retries = maxRetries
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(1<<(attempt-1))
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[DEBUG]     Retry %d/%d after %s\n", attempt, retries, delay)
			}
			time.Sleep(delay)
		}

		var reqBody io.Reader
		if len(reqBytes) > 0 {
			reqBody = bytes.NewReader(reqBytes)
		}

		req, err := http.NewRequest(method, u, reqBody)
		if err != nil {
			return nil, 0, fmt.Errorf("creating request: %w", err)
		}

		if c.AuthToken != "" {
			req.Header.Set("Authorization", "Bearer "+c.AuthToken)
		}
		req.Header.Set("User-Agent", c.UserAgent)

		if c.NoCache {
			req.Header.Set("Cache-Control", "no-cache")
		}

		if body != nil && (method == http.MethodPost || method == http.MethodPut) {
			req.Header.Set("Content-Type", "application/json")
		}

		start := time.Now()
		resp, err := c.HTTPClient.Do(req)
		elapsed := time.Since(start)

		if err != nil {
			if c.Verbose {
				fmt.Fprintf(os.Stderr, "[DEBUG] <-- ERROR after %s: %v\n", elapsed, err)
			}
			lastErr = fmt.Errorf("executing request: %w", err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
		}

		// Reset NoCache after each successful response
		c.NoCache = false

		if c.Verbose {
			fmt.Fprintf(os.Stderr, "[DEBUG] <-- %d %s (%s, %d bytes)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed, len(respBody))
			if xCache := resp.Header.Get("X-Cache"); xCache != "" {
				fmt.Fprintf(os.Stderr, "[DEBUG]     X-Cache: %s\n", xCache)
			}
			if len(respBody) > 0 {
				preview := string(respBody)
				if len(preview) > 300 {
					preview = preview[:300] + "..."
				}
				fmt.Fprintf(os.Stderr, "[DEBUG]     Body: %s\n", preview)
			}
		}

		// Check server compatibility headers (once per invocation)
		if !c.compatWarned {
			if apiVer := resp.Header.Get("X-Stompy-API-Version"); apiVer != "" {
				c.APIVersion = apiVer
			}
			if minCLI := resp.Header.Get("X-Stompy-Min-CLI-Version"); minCLI != "" {
				if warn := CheckCompat(c.Version, minCLI); warn != "" {
					fmt.Fprintln(os.Stderr, warn)
					c.compatWarned = true
				}
			}
		}

		if isRetryableStatus(resp.StatusCode) {
			lastErr = &APIError{StatusCode: resp.StatusCode, Message: http.StatusText(resp.StatusCode)}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			apiErr := &APIError{StatusCode: resp.StatusCode}
			if err := json.Unmarshal(respBody, apiErr); err != nil {
				apiErr.Message = string(respBody)
			}
			if apiErr.Message == "" {
				apiErr.Message = http.StatusText(resp.StatusCode)
			}
			return nil, resp.StatusCode, apiErr
		}

		printUsageWarning(respBody)
		return respBody, resp.StatusCode, nil
	}

	return nil, 0, lastErr
}

func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodOptions:
		return true
	}
	return false
}

func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func (c *Client) Get(path string, params url.Values, result any) error {
	data, _, err := c.Do(http.MethodGet, path, nil, params)
	if err != nil {
		return err
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func (c *Client) Post(path string, body any, result any) error {
	data, _, err := c.Do(http.MethodPost, path, body, nil)
	if err != nil {
		return err
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func (c *Client) Put(path string, body any, result any) error {
	data, _, err := c.Do(http.MethodPut, path, body, nil)
	if err != nil {
		return err
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

func (c *Client) Delete(path string, params url.Values) error {
	_, _, err := c.Do(http.MethodDelete, path, nil, params)
	return err
}

// DeleteWithResult performs a DELETE and decodes the JSON response body.
// Use for endpoints that return data (e.g. context unlock returns ContextDeleteResponse).
func (c *Client) DeleteWithResult(path string, params url.Values, result any) error {
	data, statusCode, err := c.Do(http.MethodDelete, path, nil, params)
	if err != nil {
		return err
	}
	if result != nil && statusCode != http.StatusNoContent && len(data) > 0 {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}
