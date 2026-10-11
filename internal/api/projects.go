package api

import (
	"net/url"
	"strings"
	"time"
)

type ProjectCreate struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type ProjectStats struct {
	ContextCount   int        `json:"context_count"`
	SessionCount   int        `json:"session_count"`
	FileCount      int        `json:"file_count"`
	StorageBytesDB int        `json:"storage_bytes_db"`
	StorageBytesS3 int        `json:"storage_bytes_s3"`
	LastActivity   *time.Time `json:"last_activity,omitempty"`
}

type ProjectResponse struct {
	Name        string        `json:"name"`
	SchemaName  string        `json:"schema_name"`
	CreatedAt   time.Time     `json:"created_at"`
	Role        string        `json:"role"`
	IsSystem    bool          `json:"is_system"`
	Description *string       `json:"description,omitempty"`
	Stats       *ProjectStats `json:"stats,omitempty"`
}

type ProjectListResponse struct {
	Projects []ProjectResponse `json:"projects"`
	Total    int               `json:"total"`
}

func (c *Client) ListProjects(withStats bool) (*ProjectListResponse, error) {
	params := url.Values{}
	if withStats {
		params.Set("stats", "true")
	}
	var resp ProjectListResponse
	if err := c.Get("/projects", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetProject(name string, withStats bool) (*ProjectResponse, error) {
	params := url.Values{}
	if withStats {
		params.Set("stats", "true")
	}
	var resp ProjectResponse
	if err := c.Get(projectPath(name, ""), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateProject(req ProjectCreate) (*ProjectResponse, error) {
	var resp ProjectResponse
	if err := c.Post("/projects", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) DeleteProject(name string) error {
	params := url.Values{"confirm": {"true"}}
	return c.Delete(projectPath(name, ""), params)
}

// splitOwner splits an owner-qualified project address. The canonical display
// form carries a leading "@" (`@owner/name`); the wire form the backend
// resolves is `owner/name`, so the sigil is dropped here, once, for every
// caller (STOMPY-2742).
func splitOwner(project string) (owner, name string, qualified bool) {
	return strings.Cut(strings.TrimPrefix(project, "@"), "/")
}

// projectPath is THE builder for every project-scoped route
// (/projects/{name}{suffix}). A qualified address (`@owner/name`, `owner/name`)
// becomes the bare name plus `?owner=owner` (STOMPY-2742); suffix is
// already-escaped path text beginning with "/" (or empty). Do forces any
// further params on with "&".
func projectPath(project, suffix string) string {
	owner, name, qualified := splitOwner(project)
	if !qualified {
		name = project
	}
	p := "/projects/" + url.PathEscape(name) + suffix
	if qualified {
		p += "?owner=" + url.QueryEscape(owner)
	}
	return p
}
