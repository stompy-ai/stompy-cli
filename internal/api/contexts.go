package api

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type ContextCreateRequest struct {
	Topic      string `json:"topic"`
	Content    string `json:"content"`
	Priority   string `json:"priority,omitempty"`
	Tags       string `json:"tags,omitempty"`
	ForceStore bool   `json:"force_store,omitempty"`
}

type ContextUpdateRequest struct {
	Content  string `json:"content,omitempty"`
	Priority string `json:"priority,omitempty"`
	Tags     string `json:"tags,omitempty"`
}

type ContextResponse struct {
	ID           int        `json:"id"`
	Topic        string     `json:"topic"`
	Version      string     `json:"version"`
	Priority     string     `json:"priority"`
	Tags         []string   `json:"tags"`
	Preview      *string    `json:"preview,omitempty"`
	KeyConcepts  []string   `json:"key_concepts,omitempty"`
	ContentHash  *string    `json:"content_hash,omitempty"`
	LockedAt     *time.Time `json:"locked_at,omitempty"`
	LastAccessed *time.Time `json:"last_accessed,omitempty"`
	AccessCount  int        `json:"access_count"`
}

type ContextDetailResponse struct {
	ContextResponse
	Content  string           `json:"content"`
	Versions []VersionSummary `json:"versions,omitempty"`
}

type VersionSummary struct {
	Version   string     `json:"version"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

// ContextCreateResponse decodes POST /projects/{name}/contexts (context lock).
// STOMPY-1967 item 1: the live route has no "status" field — it returns
// id/topic/version plus a dashboard url — so -o json has an object worth
// printing.
type ContextCreateResponse struct {
	ID      int    `json:"id"`
	Topic   string `json:"topic"`
	Version string `json:"version"`
	URL     string `json:"url,omitempty"`
}

// ContextDeleteResponse decodes DELETE /projects/{name}/contexts/{topic}
// (context unlock). STOMPY-1967 item 1: the live route answers
// deleted_count/versions_deleted, not "status", plus a dashboard url.
type ContextDeleteResponse struct {
	Topic           string   `json:"topic"`
	Archived        bool     `json:"archived"`
	DeletedCount    int      `json:"deleted_count,omitempty"`
	VersionsDeleted []string `json:"versions_deleted,omitempty"`
	URL             string   `json:"url,omitempty"`
}

type ContextMoveResponse struct {
	Status        string `json:"status"`
	Topic         string `json:"topic"`
	TargetProject string `json:"target_project"`
}

type ContextListResponse struct {
	Contexts []ContextResponse `json:"contexts"`
	Total    int               `json:"total"`
}

func (c *Client) ListContexts(project string, priority, tags string, limit, offset int) (*ContextListResponse, error) {
	params := url.Values{}
	if priority != "" {
		params.Set("priority", priority)
	}
	if tags != "" {
		params.Set("tags", tags)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	var resp ContextListResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/contexts", url.PathEscape(project)), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetContext(project, topic string, version string) (*ContextDetailResponse, error) {
	params := url.Values{}
	if version != "" {
		params.Set("version", version)
	}
	var resp ContextDetailResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/contexts/%s", url.PathEscape(project), url.PathEscape(topic)), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) LockContext(project string, req ContextCreateRequest) (*ContextCreateResponse, error) {
	var resp ContextCreateResponse
	if err := c.Post(fmt.Sprintf("/projects/%s/contexts", url.PathEscape(project)), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UnlockContext(project, topic string, version string, force, noArchive bool) (*ContextDeleteResponse, error) {
	params := url.Values{}
	if version != "" {
		params.Set("version", version)
	}
	if force {
		params.Set("force", "true")
	}
	if noArchive {
		params.Set("no_archive", "true")
	}

	var resp ContextDeleteResponse
	if err := c.DeleteWithResult(fmt.Sprintf("/projects/%s/contexts/%s", url.PathEscape(project), url.PathEscape(topic)), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateContext(project, topic string, req ContextUpdateRequest) (*ContextResponse, error) {
	var resp ContextResponse
	if err := c.Put(fmt.Sprintf("/projects/%s/contexts/%s", url.PathEscape(project), url.PathEscape(topic)), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SearchContexts(project, query string, limit int) (*ContextListResponse, error) {
	params := url.Values{}
	params.Set("search", query)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	var resp ContextListResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/contexts", url.PathEscape(project)), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) MoveContext(project, topic, targetProject string) (*ContextMoveResponse, error) {
	body := map[string]string{"target_project": targetProject}
	var resp ContextMoveResponse
	if err := c.Post(fmt.Sprintf("/projects/%s/contexts/%s/move", url.PathEscape(project), url.PathEscape(topic)), body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
