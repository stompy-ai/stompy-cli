package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Identity is assigned by the authenticated server, never by a request field.
type BoardPostRequest struct {
	Kind       string   `json:"kind"`
	Body       string   `json:"body"`
	AgentLabel string   `json:"agent_label,omitempty"`
	Refs       []string `json:"refs,omitempty"`
	TTLSeconds *int64   `json:"ttl_seconds,omitempty"`
}

type BoardPost struct {
	ID         int64      `json:"id"`
	AuthorID   int64      `json:"author_id"`
	AgentLabel string     `json:"agent_label"`
	Kind       string     `json:"kind"`
	Body       string     `json:"body"`
	Refs       []string   `json:"refs"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastReadAt *time.Time `json:"last_read_at,omitempty"`
	ReadCount  int64      `json:"read_count"`
	Weight     float64    `json:"weight"`
}

type BoardPostResponse struct {
	Project string    `json:"project"`
	Post    BoardPost `json:"post"`
}

type BoardReadResponse struct {
	Project string      `json:"project"`
	Posts   []BoardPost `json:"posts"`
}

func (c *Client) PostBoard(project string, request BoardPostRequest) (*BoardPostResponse, error) {
	path, params := boardPath(project)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var response BoardPostResponse
	if err := c.Post(path, request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ReadBoard(project, since, kinds string, limit int) (*BoardReadResponse, error) {
	path, params := boardPath(project)
	params.Set("limit", strconv.Itoa(limit))
	if since != "" {
		params.Set("since", since)
	}
	if kinds != "" {
		params.Set("kinds", kinds)
	}
	var response BoardReadResponse
	if err := c.Get(path, params, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func boardPath(project string) (string, url.Values) {
	params := url.Values{}
	if owner, name, qualified := strings.Cut(project, "/"); qualified {
		params.Set("owner", owner)
		project = name
	}
	return fmt.Sprintf("/projects/%s/board", url.PathEscape(project)), params
}
