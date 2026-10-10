package api

import (
	"fmt"
	"net/url"
	"time"
)

// STOMPY-2705: project API key management (REST /projects/{name}/api-key) and
// team project create (REST /shared-projects). Every call here is refused
// server-side for an API-key principal — keys cannot manage keys.

type ProjectKeyMintResponse struct {
	Secret  string `json:"secret"` // the ONLY time it exists in plaintext
	Label   string `json:"label"`
	Project string `json:"project"`
	Rotated bool   `json:"rotated"`
}

type ProjectKeyMetadata struct {
	Exists     bool       `json:"exists"`
	Label      *string    `json:"label,omitempty"`
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type ProjectKeyRequestResponse struct {
	Notified bool `json:"notified"`
}

type SharedProjectResponse struct {
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	IsShared    bool      `json:"is_shared"`
	MyRole      string    `json:"my_role"`
	CreatedAt   time.Time `json:"created_at"`
	MemberCount int       `json:"member_count"`
}

func projectKeyPath(project string) string {
	return fmt.Sprintf("/projects/%s/api-key", url.PathEscape(project))
}

// MintProjectKey mints the project's key. If one is active this ROTATES it.
func (c *Client) MintProjectKey(project, label string) (*ProjectKeyMintResponse, error) {
	var resp ProjectKeyMintResponse
	if err := c.Post(projectKeyPath(project), map[string]string{"label": label}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetProjectKey(project string) (*ProjectKeyMetadata, error) {
	var resp ProjectKeyMetadata
	if err := c.Get(projectKeyPath(project), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) RevokeProjectKey(project string) error {
	return c.Delete(projectKeyPath(project), nil)
}

func (c *Client) RequestProjectKey(project string) (*ProjectKeyRequestResponse, error) {
	var resp ProjectKeyRequestResponse
	if err := c.Post(projectKeyPath(project)+"/request", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateSharedProject creates a TEAM (born-shared) project.
func (c *Client) CreateSharedProject(name string) (*SharedProjectResponse, error) {
	var resp SharedProjectResponse
	if err := c.Post("/shared-projects", map[string]string{"name": name}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
