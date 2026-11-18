package api

type ProjectNameResult struct {
	Success       bool   `json:"success"`
	Name          string `json:"name"`
	SchemaName    string `json:"schema_name"`
	Changed       bool   `json:"changed"`
	MetadataSync  string `json:"metadata_sync,omitempty"`
	ReleasedAlias string `json:"released_alias,omitempty"`
}

type ProjectAliases struct {
	Name       string   `json:"name"`
	SchemaName string   `json:"schema_name"`
	Aliases    []string `json:"aliases"`
}

func (c *Client) RenameProject(project, newName string) (*ProjectNameResult, error) {
	var result ProjectNameResult
	if err := c.Post(projectPath(project, "/rename"), map[string]string{"new_name": newName}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetProjectAliases(project string) (*ProjectAliases, error) {
	var result ProjectAliases
	if err := c.Get(projectPath(project, "/aliases"), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
