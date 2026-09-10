package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// SearchResultMetadata is the decoded form of SearchResult.Metadata, which
// the live route encodes as a JSON-STRING (not a nested object).
type SearchResultMetadata struct {
	Priority  string   `json:"priority,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// SearchResult represents a single hit from GET /projects/{name}/search.
// STOMPY-1968: the live route names the topic `label` (there is no
// `topic`/`type`/top-level `score`/`priority` at all), and packs
// priority/tags into a JSON-encoded `metadata` STRING rather than a nested
// object — decode it with DecodedMetadata().
type SearchResult struct {
	ID            int      `json:"id"`
	Label         string   `json:"label"`
	Content       string   `json:"content,omitempty"`
	Preview       string   `json:"preview,omitempty"`
	Metadata      string   `json:"metadata,omitempty"`
	Similarity    *float64 `json:"similarity,omitempty"`
	BM25Score     *float64 `json:"bm25_score,omitempty"`
	RRFScore      *float64 `json:"rrf_score,omitempty"`
	RerankScore   *float64 `json:"rerank_score,omitempty"`
	TemporalScore *float64 `json:"temporal_score,omitempty"`
	Source        string   `json:"source,omitempty"`
	Scope         string   `json:"scope,omitempty"`
	IsChunk       bool     `json:"is_chunk,omitempty"`
	ChunkID       *int     `json:"chunk_id,omitempty"`
	ParentChunkID *int     `json:"parent_chunk_id,omitempty"`
}

// DecodedMetadata parses the JSON-encoded Metadata string. It returns the
// zero value if Metadata is empty or not valid JSON, rather than an error —
// callers use it for display, not correctness-critical logic.
func (r SearchResult) DecodedMetadata() SearchResultMetadata {
	var m SearchResultMetadata
	if r.Metadata == "" {
		return m
	}
	_ = json.Unmarshal([]byte(r.Metadata), &m)
	return m
}

// RankScore returns the best available ranking score for display: the
// combined RRF score first (when hybrid search ran both signals), then
// whichever single-signal score the route actually populated.
func (r SearchResult) RankScore() float64 {
	switch {
	case r.RRFScore != nil:
		return *r.RRFScore
	case r.RerankScore != nil:
		return *r.RerankScore
	case r.BM25Score != nil:
		return *r.BM25Score
	case r.Similarity != nil:
		return *r.Similarity
	default:
		return 0
	}
}

// SearchResponse wraps search results. STOMPY-1968: the live route answers
// `total_found`, not `total`, and does not echo the query back at all.
type SearchResponse struct {
	Results    []SearchResult `json:"results"`
	TotalFound int            `json:"total_found"`
	SearchMode string         `json:"search_mode,omitempty"`
}

// Search performs hybrid semantic + keyword search across a project.
func (c *Client) Search(project, query string, limit int) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("query", query)
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	var resp SearchResponse
	if err := c.Get(fmt.Sprintf("/projects/%s/search", project), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
