package api

import (
	"fmt"
	"net/url"
	"strconv"
)

// BugReportResponse represents a bug report. Field shape and types (id is a
// string, timestamps are strings, not int/time.Time) mirror
// src/api/routes/bug_reports.py::BugReportResponse (STOMPY-1462).
type BugReportResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Severity    string `json:"severity,omitempty"`
	Steps       string `json:"steps_to_reproduce,omitempty"`
	Expected    string `json:"expected_behavior,omitempty"`
	Actual      string `json:"actual_behavior,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// BugReportListResponse wraps a list of bug reports. The server's envelope
// key is "reports", not "bug_reports" (STOMPY-1462).
type BugReportListResponse struct {
	Reports []BugReportResponse `json:"reports"`
	Total   int                 `json:"total"`
}

// ListBugReports fetches bug reports with an optional status filter.
//
// NOTE (STOMPY-1462): the server's list endpoint is /bug-reports (not
// /projects/{project}/bug-reports — that path has never existed) and has
// no project filter at all, so `project` is accepted for CLI symmetry with
// other list commands but not sent. It also requires a dev-team X-API-Key
// header the CLI has no config surface for — see cmd/bug.go's doc comment.
func (c *Client) ListBugReports(project, status string, limit, offset int) (*BugReportListResponse, error) {
	params := url.Values{}
	if status != "" {
		params.Set("status", status)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	var resp BugReportListResponse
	if err := c.Get("/bug-reports", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetBugReport fetches a single bug report by ID (a string, not an int).
func (c *Client) GetBugReport(project string, id string) (*BugReportResponse, error) {
	var resp BugReportResponse
	if err := c.Get(fmt.Sprintf("/bug-reports/%s", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
