package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// FileResponse represents an uploaded file/document as returned by
// GET /projects/{name}/files (list items) and GET /projects/{name}/files/{id}.
// STOMPY-1967 item 7: the route answers `documents`/`title`/`size_bytes`, not
// the `files`/`filename`/`size` shape this used to decode.
type FileResponse struct {
	ID               int       `json:"id"`
	Title            string    `json:"title"`
	FileType         string    `json:"file_type,omitempty"`
	MimeType         string    `json:"mime_type,omitempty"`
	S3URL            string    `json:"s3_url,omitempty"`
	SizeBytes        int       `json:"size_bytes"`
	ProcessingStatus string    `json:"processing_status,omitempty"`
	UploadedAt       time.Time `json:"uploaded_at,omitempty"`
	AIDescription    *string   `json:"ai_description,omitempty"`
}

// FileListResponse wraps a list of files (documents) for a project.
type FileListResponse struct {
	Documents []FileResponse `json:"documents"`
	Total     int            `json:"total"`
	Limit     int            `json:"limit,omitempty"`
	Offset    int            `json:"offset,omitempty"`
}

// FileMetadata is the nested metadata object the upload endpoint returns —
// size and type live here, not at the top level, unlike the list/get shape.
type FileMetadata struct {
	FileType  string `json:"file_type,omitempty"`
	SizeBytes int    `json:"size_bytes"`
	Priority  string `json:"priority,omitempty"`
}

// FileUploadResponse decodes POST /projects/{name}/files. Distinct from
// FileResponse: size/type are nested in `metadata`, the timestamp key is
// `created_at` (not `uploaded_at`), and there's a `message` field.
type FileUploadResponse struct {
	ID               int          `json:"id"`
	Title            string       `json:"title"`
	CreatedAt        time.Time    `json:"created_at"`
	Metadata         FileMetadata `json:"metadata"`
	S3URL            string       `json:"s3_url,omitempty"`
	ProcessingStatus string       `json:"processing_status,omitempty"`
	ContextID        *int         `json:"context_id,omitempty"`
	AIDescription    *string      `json:"ai_description,omitempty"`
	Message          string       `json:"message,omitempty"`
}

// ListFiles fetches files for a project with optional search.
func (c *Client) ListFiles(project, search string, limit, offset int) (*FileListResponse, error) {
	params := url.Values{}
	if search != "" {
		params.Set("search", search)
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}

	var resp FileListResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/files", project), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetFile fetches a single file by ID.
func (c *Client) GetFile(project string, id int) (*FileResponse, error) {
	var resp FileResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/files/%d", project, id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UploadFile uploads a file with multipart form data.
func (c *Client) UploadFile(project, filePath, label string) (*FileUploadResponse, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("copying file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	u := c.BaseURL + fmt.Sprintf("/projects/%s/files", project)
	if label != "" {
		// The route reads `label` as a query parameter, not a form field.
		u += "?" + url.Values{"label": {label}}.Encode()
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] --> POST %s (multipart, file: %s)\n", u, filePath)
	}

	req, err := http.NewRequest(http.MethodPost, u, &body)
	if err != nil {
		return nil, fmt.Errorf("creating upload request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("User-Agent", c.UserAgent)
	if c.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AuthToken)
	}

	start := time.Now()
	resp, err := c.HTTPClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("executing upload request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading upload response: %w", err)
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] <-- %d %s (%s, %d bytes)\n", resp.StatusCode, http.StatusText(resp.StatusCode), elapsed, len(respBody))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if err := json.Unmarshal(respBody, apiErr); err != nil {
			apiErr.Message = string(respBody)
		}
		return nil, apiErr
	}

	var fileResp FileUploadResponse
	if err := json.Unmarshal(respBody, &fileResp); err != nil {
		return nil, fmt.Errorf("decoding upload response: %w", err)
	}
	return &fileResp, nil
}

// DeleteFile deletes a file by ID.
func (c *Client) DeleteFile(project string, id int) error {
	return c.Delete(fmt.Sprintf("/projects/%s/files/%d", project, id), nil)
}
