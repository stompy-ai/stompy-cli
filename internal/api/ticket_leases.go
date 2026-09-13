package api

import (
	"fmt"
	"net/url"
	"strings"
)

type LeaseHolder struct {
	AccountID  int64  `json:"account_id"`
	AgentLabel string `json:"agent_label"`
}

type TicketLeaseRequest struct {
	AgentLabel string `json:"agent_label"`
	TTLMinutes int    `json:"ttl_minutes,omitempty"`
	Assignee   string `json:"assignee,omitempty"`
}

type TicketLeaseResponse struct {
	Status  string          `json:"status"`
	Changed bool            `json:"changed"`
	Via     string          `json:"via,omitempty"`
	Ticket  *TicketResponse `json:"ticket"`
}

func (c *Client) LeaseTicket(project, action string, id int, request TicketLeaseRequest) (*TicketLeaseResponse, error) {
	suffix := "claim-next"
	switch action {
	case "claim", "release":
		suffix = fmt.Sprintf("%d/%s", id, action)
	case "claim-next":
	default:
		return nil, fmt.Errorf("unknown lease action: %s", action)
	}
	params := url.Values{}
	if owner, name, qualified := strings.Cut(project, "/"); qualified {
		params.Set("owner", owner)
		project = name
	}
	path := fmt.Sprintf("/projects/%s/tickets/%s", url.PathEscape(project), suffix)
	if len(params) > 0 {
		path += "?" + params.Encode()
	}
	var response TicketLeaseResponse
	if err := c.Post(path, request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
