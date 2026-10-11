package api

import (
	"fmt"
)

type TicketArchiveResponse struct {
	Status string          `json:"status"`
	Ticket *TicketResponse `json:"ticket"`
}

func archivePath(project, suffix string) string {
	return projectPath(project, fmt.Sprintf("/tickets/%s", suffix))
}

func (c *Client) ArchiveTicket(project, action string, id int) (*TicketArchiveResponse, error) {
	if (action != "archive" && action != "unarchive") || id <= 0 {
		return nil, fmt.Errorf("invalid targeted archive request")
	}
	var response TicketArchiveResponse
	if err := c.Post(archivePath(project, fmt.Sprintf("%d/%s", id, action)), nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) BatchArchiveTickets(project string, ids []int, confirm bool) (map[string]interface{}, error) {
	var response map[string]interface{}
	request := map[string]interface{}{"ticket_ids": ids, "confirm": confirm}
	if err := c.Post(archivePath(project, "batch/archive"), request, &response); err != nil {
		return nil, err
	}
	return response, nil
}
