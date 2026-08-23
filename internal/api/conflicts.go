package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// ConflictItem is one side of a conflict (STOMPY-1462: matches the backend's
// nested item_a/item_b shape, not a flat "topic" pair).
type ConflictItem struct {
	ItemType    string   `json:"item_type"`
	ItemID      int      `json:"item_id"`
	Excerpt     string   `json:"excerpt,omitempty"`
	FullContent string   `json:"full_content,omitempty"`
	CreatedAt   *float64 `json:"created_at,omitempty"`
}

// ConflictResponse represents a detected conflict between contexts.
// Field shape and types (id is a string UUID; timestamps are epoch floats)
// mirror src/api/routes/conflicts.py::ConflictResponse exactly — the CLI's
// previous flat ContextATopic/ContextBTopic shape never matched what the
// server returns (STOMPY-1462).
type ConflictResponse struct {
	ID                string       `json:"id"`
	ProjectID         string       `json:"project_id"`
	ItemA             ConflictItem `json:"item_a"`
	ItemB             ConflictItem `json:"item_b"`
	DetectionMethod   string       `json:"detection_method"`
	Confidence        float64      `json:"confidence"`
	ContradictionType string       `json:"contradiction_type"`
	Status            string       `json:"status"`
	Resolution        *string      `json:"resolution,omitempty"`
	ResolvedBy        *string      `json:"resolved_by,omitempty"`
	ResolvedAt        *float64     `json:"resolved_at,omitempty"`
	ResolutionNotes   *string      `json:"resolution_notes,omitempty"`
	CreatedAt         *float64     `json:"created_at,omitempty"`
}

// ConflictListResponse wraps a list of conflicts.
type ConflictListResponse struct {
	Conflicts     []ConflictResponse `json:"conflicts"`
	Total         int                `json:"total"`
	PendingCount  int                `json:"pending_count"`
	ResolvedCount int                `json:"resolved_count"`
}

// ConflictDetectRequest triggers conflict detection.
type ConflictDetectRequest struct {
	Scope string `json:"scope,omitempty"` // "all", "recent", or "specific_ids"
}

// ConflictDetectResponse is the result of a synchronous (scope != "all")
// detection run.
type ConflictDetectResponse struct {
	ConflictsFound   int     `json:"conflicts_found"`
	AutoResolved     int     `json:"auto_resolved"`
	Pending          int     `json:"pending"`
	ProcessingTimeMs float64 `json:"processing_time_ms"`
}

// ConflictDetectQueuedResponse is returned (HTTP 202) for scope="all", which
// the server queues as a background job instead of blocking the request
// (STOMPY-1389 — a full scan can run minutes). ConflictsFound is not known
// yet; poll `conflict list` for results.
type ConflictDetectQueuedResponse struct {
	Status  string `json:"status"`
	JobID   string `json:"job_id"`
	Scope   string `json:"scope"`
	Message string `json:"message"`
}

// ConflictResolveRequest resolves a conflict.
type ConflictResolveRequest struct {
	Resolution string `json:"resolution"` // "dismiss", "keep_a", "keep_b", "merge"
}

// ListConflicts fetches conflicts with optional status filter.
// project_id is a QUERY PARAM on the server (GET /conflicts?project_id=...),
// not a path segment — /projects/{project}/conflicts has never existed
// (STOMPY-1462).
func (c *Client) ListConflicts(project, status string, limit, offset int) (*ConflictListResponse, error) {
	params := url.Values{}
	params.Set("project_id", project)
	if status != "" {
		params.Set("status", status)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	var resp ConflictListResponse
	if err := c.Get("/conflicts", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetConflict fetches a single conflict by ID (a string UUID, not an int).
func (c *Client) GetConflict(project string, id string) (*ConflictResponse, error) {
	params := url.Values{"project_id": {project}}
	var resp ConflictResponse
	if err := c.Get(fmt.Sprintf("/conflicts/%s", id), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DetectConflicts triggers conflict detection. For scope="all" the server
// queues the scan and returns HTTP 202 with a job handle instead of the
// completed-scan summary (STOMPY-1389); queued is non-nil in that case.
func (c *Client) DetectConflicts(project string, req ConflictDetectRequest) (resp *ConflictDetectResponse, queued *ConflictDetectQueuedResponse, err error) {
	params := url.Values{"project_id": {project}}
	data, statusCode, err := c.Do(http.MethodPost, "/conflicts/detect", req, params)
	if err != nil {
		return nil, nil, err
	}

	if statusCode == http.StatusAccepted {
		queued = &ConflictDetectQueuedResponse{}
		if err := json.Unmarshal(data, queued); err != nil {
			return nil, nil, fmt.Errorf("decoding queued response: %w", err)
		}
		return nil, queued, nil
	}

	resp = &ConflictDetectResponse{}
	if err := json.Unmarshal(data, resp); err != nil {
		return nil, nil, fmt.Errorf("decoding response: %w", err)
	}
	return resp, nil, nil
}

// ResolveConflict resolves a conflict by ID (a string UUID, not an int).
func (c *Client) ResolveConflict(project string, id string, req ConflictResolveRequest) (*ConflictResponse, error) {
	params := url.Values{"project_id": {project}}
	var resp ConflictResponse
	path := fmt.Sprintf("/conflicts/%s/resolve?%s", id, params.Encode())
	if err := c.Post(path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
