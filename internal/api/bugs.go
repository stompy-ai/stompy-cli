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
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Severity    string         `json:"severity,omitempty"`
	Steps       string         `json:"steps_to_reproduce,omitempty"`
	Expected    string         `json:"expected_behavior,omitempty"`
	Actual      string         `json:"actual_behavior,omitempty"`
	ProjectName string         `json:"project_name,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Environment map[string]any `json:"environment,omitempty"`
	CreatedAt   string         `json:"created_at,omitempty"`
	UpdatedAt   string         `json:"updated_at,omitempty"`
}

// BugReportListResponse wraps a list of bug reports. The server's envelope
// key is "reports", not "bug_reports" (STOMPY-1462).
type BugReportListResponse struct {
	Reports     []BugReportResponse   `json:"reports"`
	Total       int                   `json:"total"`
	Quarantined []BugReportQuarantine `json:"quarantined,omitempty"`
}

// BugReportQuarantine names one report on this page that the server could not
// show, so the page is never silently short (STOMPY-3848).
type BugReportQuarantine struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// ListBugReports fetches bug reports with an optional status filter.
//
// NOTE (STOMPY-1462): the server's list endpoint is /bug-reports (not
// /projects/{project}/bug-reports — that path has never existed) and has
// no project filter at all, so `project` is accepted for CLI symmetry with
// other list commands but not sent. The ordinary bearer key reads the
// caller's own reports (staff: all) since STOMPY-3834.
func (c *Client) ListBugReports(project, status string, limit, offset int) (*BugReportListResponse, error) {
	return c.ListBugReportsWithSeverity(project, status, "", limit, offset)
}

// ListBugReportsWithSeverity composes canonical severity, status and pagination.
// The unfiltered wrapper preserves existing callers; project is still not sent.
func (c *Client) ListBugReportsWithSeverity(project, status, severity string, limit, offset int) (*BugReportListResponse, error) {
	params := url.Values{}
	if severity != "" {
		params.Set("severity", severity)
	}
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
