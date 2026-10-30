package api

import (
	"net/url"
	"strconv"
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
	// Read evidence by kind (STOMPY-3182) and supersession; absent from older servers.
	SupersededAt   *time.Time `json:"superseded_at,omitempty"`
	DeliveredCount *int64     `json:"delivered_count,omitempty"`
	ViewedCount    *int64     `json:"viewed_count,omitempty"`
	LastViewedAt   *time.Time `json:"last_viewed_at,omitempty"`
	// Author fields (STOMPY-3178), set by the server. Name and role are null when
	// the server could not resolve them, so they keep their null in JSON.
	// minimal: an older server's answer also renders them as null here; revisit
	// only if a script needs to tell "older server" from "unresolved".
	AuthorKind string  `json:"author_kind,omitempty"`
	AuthorName *string `json:"author_name"`
	AuthorRole *string `json:"author_role"`
	IsCaller   *bool   `json:"is_caller,omitempty"`
}

type BoardPostResponse struct {
	Project string    `json:"project"`
	Post    BoardPost `json:"post"`
	// The status-slot answer (STOMPY-3172): the label the post landed under, and
	// a notice when it replaced a live status this door cannot attribute.
	AgentLabel             *string `json:"agent_label,omitempty"`
	SupersededOtherSession *bool   `json:"superseded_other_session,omitempty"`
	Notice                 string  `json:"notice,omitempty"`
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
	return projectPath(project, "/board"), url.Values{}
}
