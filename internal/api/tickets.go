package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type TicketCreate struct {
	Title       string         `json:"title"`
	Description *string        `json:"description,omitempty"`
	Type        string         `json:"type"`
	Priority    string         `json:"priority"`
	Assignee    *string        `json:"assignee,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type TicketUpdate struct {
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Priority    *string  `json:"priority,omitempty"`
	Assignee    *string  `json:"assignee,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type TicketResponse struct {
	ID          int              `json:"id"`
	Title       string           `json:"title"`
	Description *string          `json:"description,omitempty"`
	Type        string           `json:"type"`
	Status      string           `json:"status"`
	Priority    string           `json:"priority"`
	Assignee    *string          `json:"assignee,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	CreatedAt   *float64         `json:"created_at,omitempty"`
	UpdatedAt   *float64         `json:"updated_at,omitempty"`
	ClosedAt    *float64         `json:"closed_at,omitempty"`
	History     []TicketHistory  `json:"history,omitempty"`
	Links       []TicketLinkResp `json:"links,omitempty"`
	URL         string           `json:"url,omitempty"`
}

type TicketHistory struct {
	Action    string  `json:"action"`
	Field     string  `json:"field,omitempty"`
	OldValue  *string `json:"old_value,omitempty"`
	NewValue  *string `json:"new_value,omitempty"`
	Timestamp float64 `json:"timestamp"`
}

// TicketListResponse decodes GET /projects/{name}/tickets. The route answers
// the kanban BOARD shape ({"columns":[{"status","count","tickets":[...]}],
// "total":N}) — STOMPY-1967 item 6. A flat {"tickets":[...],"total":N} shape
// (used by /tickets/search) is also accepted so this type stays reusable.
type TicketListResponse struct {
	Tickets []TicketResponse
	Total   int
}

func (r *TicketListResponse) UnmarshalJSON(b []byte) error {
	var raw struct {
		Tickets []TicketResponse `json:"tickets"`
		Columns []BoardColumn    `json:"columns"`
		Total   int              `json:"total"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	r.Total = raw.Total
	if len(raw.Columns) > 0 {
		for _, col := range raw.Columns {
			r.Tickets = append(r.Tickets, col.Tickets...)
		}
		return nil
	}
	r.Tickets = raw.Tickets
	return nil
}

type TicketSearchResponse struct {
	Results []TicketResponse `json:"tickets"`
	Total   int              `json:"total"`
	Query   string           `json:"query"`
}

type BoardColumn struct {
	Status  string           `json:"status"`
	Count   int              `json:"count"`
	Tickets []TicketResponse `json:"tickets"`
}

type BoardView struct {
	Columns []BoardColumn `json:"columns"`
	Total   int           `json:"total"`
}

type LinkCreate struct {
	TargetID int    `json:"target_id"`
	LinkType string `json:"link_type"`
}

type TicketLinkResp struct {
	ID           int    `json:"id"`
	SourceID     int    `json:"source_id"`
	TargetID     int    `json:"target_id"`
	LinkType     string `json:"link_type"`
	TargetTitle  string `json:"target_title,omitempty"`
	TargetStatus string `json:"target_status,omitempty"`
}

type TransitionRequest struct {
	Status string `json:"status"`
}

func (c *Client) ListTickets(project string, status, ticketType, priority string, limit, offset int) (*TicketListResponse, error) {
	params := url.Values{}
	if status != "" {
		params.Set("status", status)
	}
	if ticketType != "" {
		params.Set("type", ticketType)
	}
	if priority != "" {
		params.Set("priority", priority)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	var resp TicketListResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/tickets", url.PathEscape(project)), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetTicket(project string, id int) (*TicketResponse, error) {
	var resp TicketResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/tickets/%d", url.PathEscape(project), id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) CreateTicket(project string, req TicketCreate) (*TicketResponse, error) {
	var resp TicketResponse
	if err := c.Post(fmt.Sprintf("/projects/%s/tickets", url.PathEscape(project)), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) UpdateTicket(project string, id int, req TicketUpdate) (*TicketResponse, error) {
	var resp TicketResponse
	if err := c.Put(fmt.Sprintf("/projects/%s/tickets/%d", url.PathEscape(project), id), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) TransitionTicket(project string, id int, status string) (*TicketResponse, error) {
	body := TransitionRequest{Status: status}
	var resp TicketResponse
	if err := c.Post(fmt.Sprintf("/projects/%s/tickets/%d/move", url.PathEscape(project), id), body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) SearchTickets(project, query string, ticketType, status string, limit int) (*TicketSearchResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	if ticketType != "" {
		params.Set("type", ticketType)
	}
	if status != "" {
		params.Set("status", status)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	var resp TicketSearchResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/tickets/search", url.PathEscape(project)), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetBoard(project string, view, ticketType, status string) (*BoardView, error) {
	params := url.Values{}
	if view != "" {
		params.Set("view", view)
	}
	if ticketType != "" {
		params.Set("type", ticketType)
	}
	if status != "" {
		params.Set("status", status)
	}
	var resp BoardView
	if err := c.Get(fmt.Sprintf("/projects/%s/tickets/board", url.PathEscape(project)), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) AddLink(project string, ticketID int, req LinkCreate) (*TicketLinkResp, error) {
	var resp TicketLinkResp
	if err := c.Post(fmt.Sprintf("/projects/%s/tickets/%d/links", url.PathEscape(project), ticketID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) ListLinks(project string, ticketID int) ([]TicketLinkResp, error) {
	var resp []TicketLinkResp
	if err := c.Get(fmt.Sprintf("/projects/%s/tickets/%d/links", url.PathEscape(project), ticketID), nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) RemoveLink(project string, ticketID, linkID int) error {
	return c.Delete(fmt.Sprintf("/projects/%s/tickets/%d/links/%d", url.PathEscape(project), ticketID, linkID), nil)
}
