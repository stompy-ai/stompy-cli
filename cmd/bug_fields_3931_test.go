package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jedib0t/go-pretty/v6/text"
	"gopkg.in/yaml.v3"
)

func assertBug3931HumanField(t *testing.T, out, label, value string) {
	t.Helper()
	for _, line := range strings.Split(text.StripEscape(out), "\n") {
		key, got, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(key) == label && strings.TrimSpace(got) == value {
			return
		}
	}
	t.Errorf("%s must show %q on its own line: %s", label, value, out)
}

func TestBug3931_ReporterAndOccurrences(t *testing.T) {
	for _, actor := range []string{"owned", "anonymous"} {
		for _, format := range []string{"json", "yaml", "table"} {
			t.Run(actor+"/"+format, func(t *testing.T) {
				reporter, count := "2147483647", 1
				var wantReporter any = float64(2147483647)
				if actor == "anonymous" {
					reporter, count, wantReporter = `"anonymous"`, 3, "anonymous"
				}
				body := fmt.Sprintf(`{"id":"BUG-3931","status":"triage","severity":"high","reporter_user_id":%s,"occurrences":%d,"user_email":"private@example.invalid"}`, reporter, count)
				out, requests, err := runCLI(t, replyJSON(200, body), "-o", format, "bug", "get", "BUG-3931")
				if err != nil || len(requests) != 1 || requests[0].path != "/bug-reports/BUG-3931" {
					t.Fatalf("get: %v %+v %s", err, requests, out)
				}
				if strings.Contains(out, "private@example.invalid") {
					t.Fatalf("contact appeared in report output: %s", out)
				}
				if format == "table" {
					assertBug3931HumanField(t, out, "Reporter", strings.Trim(reporter, `"`))
					assertBug3931HumanField(t, out, "Occurrences", fmt.Sprint(count))
					return
				}
				if format == "yaml" {
					var raw map[string]any
					if err := yaml.Unmarshal([]byte(out), &raw); err != nil {
						t.Fatal(err)
					}
					if actor == "owned" {
						if value, ok := raw["Reporter"].(int); !ok || value != 2147483647 {
							t.Fatalf("YAML reporter must stay an integer, got %T %v", raw["Reporter"], raw["Reporter"])
						}
					}
					if value, ok := raw["Occurrences"].(int); !ok || value != count {
						t.Fatalf("YAML occurrences must stay an integer: %#v", raw)
					}
				}
				got := decodeBug3867(t, format, out)
				if !reflect.DeepEqual(got["Reporter"], wantReporter) || got["Occurrences"] != float64(count) {
					t.Fatalf("reporter/count changed type or value: %#v", got)
				}
			})
		}
	}
}

func TestBug3931_MissingObservationIsNotInvented(t *testing.T) {
	for _, variant := range []string{"absent", "null"} {
		for _, format := range []string{"json", "yaml", "table"} {
			t.Run(variant+"/"+format, func(t *testing.T) {
				extra := ""
				if variant == "null" {
					extra = `,"reporter_user_id":null,"occurrences":null`
				}
				out, _, err := runCLI(t, replyJSON(200, `{"id":"BUG-1","status":"triage"`+extra+`}`), "-o", format, "bug", "get", "BUG-1")
				if err != nil {
					t.Fatal(err)
				}
				if format == "table" {
					for _, label := range []string{"Reporter", "Occurrences"} {
						assertBug3931HumanField(t, out, label, "(not recorded)")
					}
					return
				}
				got := decodeBug3867(t, format, out)
				for _, key := range []string{"Reporter", "Occurrences"} {
					if value, exists := got[key]; !exists || value != nil {
						t.Errorf("missing data must stay explicit null, %s=%#v in %#v", key, value, got)
					}
				}
			})
		}
	}
}

func TestBug3931_ListContext(t *testing.T) {
	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			body := `{"reports":[{"id":"BUG-1","title":"owned","status":"parked","severity":"low","project_name":"demo","created_at":"2026-11-24T01:02:03Z","updated_at":"2026-11-28T07:30:00Z"}],"total":1}`
			out, requests, err := runCLI(t, replyJSON(200, body), "-o", format, "bug", "list")
			if err != nil || len(requests) != 1 || requests[0].path != "/bug-reports" {
				t.Fatalf("list: %v %+v %s", err, requests, out)
			}
			if format == "table" {
				for _, value := range []string{"PROJECT", "demo", "UPDATED", "2026-11-28", "BUG-1", "parked", "low"} {
					if !strings.Contains(out, value) {
						t.Errorf("missing %q: %s", value, out)
					}
				}
				return
			}
			var rows []map[string]any
			var decodeErr error
			if format == "yaml" {
				decodeErr = yaml.Unmarshal([]byte(out), &rows)
			} else {
				decodeErr = json.Unmarshal([]byte(out), &rows)
			}
			if decodeErr != nil || len(rows) != 1 {
				t.Fatalf("list must remain a row array: %v %s", decodeErr, out)
			}
			for key, value := range map[string]any{"ID": "BUG-1", "TITLE": "owned", "STATUS": "parked", "SEVERITY": "low", "CREATED": "2026-11-24T01:02:03Z", "PROJECT": "demo", "UPDATED": "2026-11-28T07:30:00Z"} {
				if !reflect.DeepEqual(rows[0][key], value) {
					t.Errorf("%s=%#v, want %#v", key, rows[0][key], value)
				}
			}
		})
	}
}

func TestBug3931_RefusalControl(t *testing.T) {
	out, requests, err := runCLI(t, replyJSON(404, `{"detail":"No bug report visible to you."}`), "bug", "get", "BUG-7")
	if err == nil || len(requests) != 1 || out != "" || !strings.Contains(err.Error(), "404") {
		t.Fatalf("refusal became a report: %v %+v %q", err, requests, out)
	}
}
