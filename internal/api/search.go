package api

import (
	"fmt"
	"net/url"
	"strconv"
)

// SearchResult represents a single search hit.
type SearchResult struct {
	ID       int     `json:"id"`
	Topic    string  `json:"topic"`
	Type     string  `json:"type"` // "context", "ticket", "file"
	Preview  string  `json:"preview"`
	Score    float64 `json:"score,omitempty"`
	Priority string  `json:"priority,omitempty"`
}

// SearchResponse wraps search results.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
	Total   int            `json:"total"`
	Query   string         `json:"query"`
}

// Search performs hybrid semantic + keyword search across a project.
func (c *Client) Search(project, query string, limit int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	var resp SearchResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/search", project), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
