package cmd

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func decodeBug3867(t *testing.T, format, text string) map[string]any {
	t.Helper()
	var got map[string]any
	var err error
	if format == "yaml" {
		err = yaml.Unmarshal([]byte(text), &got)
		if err == nil {
			raw, _ := json.Marshal(got)
			err = json.Unmarshal(raw, &got)
		}
	} else {
		err = json.Unmarshal([]byte(text), &got)
	}
	if err != nil {
		t.Fatalf("decode %s: %v: %q", format, err, text)
	}
	return got
}

func TestBug3867_CanonicalContextFields(t *testing.T) {
	for _, format := range []string{"json", "yaml", "table"} {
		t.Run(format, func(t *testing.T) {
			body := `{"id":"BUG-3867","title":"owned","status":"parked","severity":"high",` +
				`"project_name":"alice/project","session_id":"s-3867","updated_at":"2026-11-25T07:30:00Z",` +
				`"environment":{"source":"cli","nested":{"n":2,"flag":true},"empty":null}}`
			out, requests, err := runCLI(t, replyJSON(200, body), "-o", format, "bug", "get", "BUG-3867")
			if err != nil || len(requests) != 1 || requests[0].path != "/bug-reports/BUG-3867" {
				t.Fatalf("get: %v %+v", err, requests)
			}
			if format == "table" {
				for _, want := range []string{"BUG-3867", "parked", "high", "Project Name", "alice/project", "Session ID", "s-3867", "Updated", "2026-11-25T07:30:00Z", "Environment", `"source": "cli"`, `"n": 2`, `"flag": true`} {
					if !strings.Contains(out, want) {
						t.Errorf("missing %q: %s", want, out)
					}
				}
				return
			}
			got := decodeBug3867(t, format, out)
			want := map[string]any{"ID": "BUG-3867", "Status": "parked", "Severity": "high", "Project Name": "alice/project", "Session ID": "s-3867", "Updated": "2026-11-25T07:30:00Z", "Environment": map[string]any{"source": "cli", "nested": map[string]any{"n": float64(2), "flag": true}, "empty": nil}}
			for key, value := range want {
				if !reflect.DeepEqual(got[key], value) {
					t.Errorf("%s = %#v, want %#v (%s)", key, got[key], value, out)
				}
			}
		})
	}
}

func TestBug3867_AbsentContextPolicy(t *testing.T) {
	for _, variant := range []string{"absent", "null", "empty_object"} {
		for _, format := range []string{"json", "yaml", "table"} {
			t.Run(variant+"/"+format, func(t *testing.T) {
				extra := ""
				if variant != "absent" {
					extra = `,"project_name":null,"session_id":null,"updated_at":null,"environment":null`
					if variant == "empty_object" {
						extra = strings.Replace(extra, `"environment":null`, `"environment":{}`, 1)
					}
				}
				out, _, err := runCLI(t, replyJSON(200, `{"id":"BUG-1","status":"triage","severity":"low"`+extra+`}`), "-o", format, "bug", "get", "BUG-1")
				if err != nil {
					t.Fatal(err)
				}
				if format == "table" {
					for _, label := range []string{"Project Name", "Session ID", "Updated", "Environment", "(not recorded)"} {
						if !strings.Contains(out, label) {
							t.Errorf("missing %q: %s", label, out)
						}
					}
					if variant == "empty_object" && !strings.Contains(out, "{}") {
						t.Fatalf("empty environment object lost: %s", out)
					}
					return
				}
				got := decodeBug3867(t, format, out)
				for _, key := range []string{"Project Name", "Session ID", "Updated"} {
					if value, exists := got[key]; !exists || value != "" {
						t.Errorf("absent %s must be an explicit empty string: %#v", key, got)
					}
				}
				value, exists := got["Environment"]
				var want any
				if variant == "empty_object" {
					want = map[string]any{}
				}
				if !exists || !reflect.DeepEqual(value, want) {
					t.Fatalf("environment = %#v, want %#v: %s", value, want, out)
				}
			})
		}
	}
}

func TestBug3867_RefusalControl(t *testing.T) {
	out, requests, err := runCLI(t, replyJSON(404, `{"detail":"not found"}`), "bug", "get", "BUG-8")
	if err == nil || len(requests) != 1 || strings.Contains(out, "Environment") || !strings.Contains(fmt.Sprint(err), "404") {
		t.Fatalf("refused get: %v %+v %q", err, requests, out)
	}
}
