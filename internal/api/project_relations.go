package api

import "net/url"

// STOMPY-2712: project KG metadata (PATCH /projects/{name}) and project
// relations (/projects/{name}/edges). Responses are returned as the decoded
// API object so `-o json` prints what the server sent, not a re-model of it.
// A foreign qualified address is refused BY THE SERVER (STOMPY-2746); these
// calls send it as ?owner= via projectPath and surface that refusal intact.

// ProjectMetadataUpdate is the PATCH body: a nil field is omitted (unchanged),
// a name in Clear is unset.
type ProjectMetadataUpdate struct {
	Origin         *string  `json:"origin,omitempty"`
	Category       *string  `json:"category,omitempty"`
	LifecycleState *string  `json:"lifecycle_state,omitempty"`
	Purpose        *string  `json:"purpose,omitempty"`
	Intent         *string  `json:"intent,omitempty"`
	Clear          []string `json:"clear,omitempty"`
}

type ProjectEdgeCreate struct {
	Target       string  `json:"target"`
	RelationType string  `json:"relation_type"`
	Note         *string `json:"note,omitempty"`
	Grounding    *string `json:"grounding,omitempty"`
}

func (c *Client) SetProjectMetadata(project string, req ProjectMetadataUpdate) (map[string]any, error) {
	var resp map[string]any
	if err := c.Patch(projectPath(project, ""), req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) LinkProject(project string, req ProjectEdgeCreate) (map[string]any, error) {
	var resp map[string]any
	if err := c.Post(projectPath(project, "/edges"), req, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// UnlinkProject removes relation(s) project -> target; an empty relationType
// removes every relation to target.
func (c *Client) UnlinkProject(project, target, relationType string) (map[string]any, error) {
	params := url.Values{"target": {target}}
	if relationType != "" {
		params.Set("relation_type", relationType)
	}
	var resp map[string]any
	if err := c.DeleteWithResult(projectPath(project, "/edges"), params, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) ListProjectEdges(project string) (map[string]any, error) {
	var resp map[string]any
	if err := c.Get(projectPath(project, "/edges"), nil, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}
