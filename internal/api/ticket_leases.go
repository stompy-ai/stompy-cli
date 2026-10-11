package api

import (
	"fmt"
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
	path := projectPath(project, fmt.Sprintf("/tickets/%s", suffix))
	var response TicketLeaseResponse
	if err := c.Post(path, request, &response); err != nil {
		return nil, err
	}
	return &response, nil
}
