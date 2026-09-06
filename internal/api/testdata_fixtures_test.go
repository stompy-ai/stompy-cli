package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestTestdataFixturesDecode is a regression guard for the whole class of
// bugs STOMPY-1967/1968 fixed: each *.json file under testdata/ is a
// RECORDED live response, and this table asserts the CLI's decoder for that
// route still parses it into something non-empty. When a route reshapes
// again, this goes red before a human has to rediscover it via dogfood —
// and forgetting to wire up a new fixture's case is itself a failure, so
// the next person who drops a fixture in here is told to finish the job.
func TestTestdataFixturesDecode(t *testing.T) {
	cases := map[string]func(t *testing.T, body []byte){
		"ticket_list_board.json": func(t *testing.T, body []byte) {
			var resp TicketListResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("Unmarshal() error: %v", err)
			}
			if len(resp.Tickets) == 0 {
				t.Error("TicketListResponse decoded zero tickets from a fixture with one")
			}
		},
		"file_list.json": func(t *testing.T, body []byte) {
			var resp FileListResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("Unmarshal() error: %v", err)
			}
			if len(resp.Documents) == 0 {
				t.Error("FileListResponse decoded zero documents from a fixture with one")
			}
		},
		"file_upload.json": func(t *testing.T, body []byte) {
			var resp FileUploadResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("Unmarshal() error: %v", err)
			}
			if resp.ID == 0 || resp.Title == "" || resp.Metadata.SizeBytes == 0 {
				t.Errorf("FileUploadResponse decoded incompletely: %+v", resp)
			}
		},
		"search_results.json": func(t *testing.T, body []byte) {
			var resp SearchResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("Unmarshal() error: %v", err)
			}
			if resp.TotalFound == 0 || len(resp.Results) == 0 {
				t.Fatalf("SearchResponse decoded incompletely: %+v", resp)
			}
			if resp.Results[0].Label == "" {
				t.Error("SearchResult.Label is empty")
			}
			if resp.Results[0].DecodedMetadata().Priority == "" {
				t.Error("SearchResult.DecodedMetadata().Priority is empty")
			}
		},
	}

	entries, err := os.ReadDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		seen[e.Name()] = true
		check, ok := cases[e.Name()]
		if !ok {
			t.Errorf("testdata/%s has no decode-regression case in TestTestdataFixturesDecode — add one", e.Name())
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("testdata", e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			check(t, body)
		})
	}

	for name := range cases {
		if !seen[name] {
			t.Errorf("case registered for testdata/%s but the fixture file is missing", name)
		}
	}
}
