package cmd

import (
	"github.com/stompy-ai/stompy-cli/internal/api"
	"github.com/stompy-ai/stompy-cli/internal/output"
)

// Machine output retains environment as an object (or null), never a JSON string.
// Missing context strings are explicit empty values; human output names them.
func formatBugDetail(formatter output.Formatter, fields []output.KeyValue, report *api.BugReportResponse) string {
	context := []output.KeyValue{
		{Key: "Project Name", Value: report.ProjectName},
		{Key: "Session ID", Value: report.SessionID},
		{Key: "Updated", Value: report.UpdatedAt},
	}
	if isTableOutput() {
		for i := range context {
			if context[i].Value == "" {
				context[i].Value = "(not recorded)"
			}
		}
	}
	fields = append(fields, context...)
	if !isTableOutput() {
		obj := make(map[string]any, len(fields)+1)
		for _, field := range fields {
			obj[field.Key] = field.Value
		}
		if report.Environment == nil {
			obj["Environment"] = nil
		} else {
			obj["Environment"] = report.Environment
		}
		return formatter.FormatRaw(obj)
	}
	environment := "(not recorded)"
	if report.Environment != nil {
		environment = output.NewFormatter("json").FormatRaw(report.Environment)
	}
	fields = append(fields, output.KeyValue{Key: "Environment", Value: environment})
	return formatter.FormatSingle(fields)
}
