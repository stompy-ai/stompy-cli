package cmd

import (
	"fmt"
	"math"
	"strconv"

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
	// A missing observation is unknown, never a fabricated reporter or count.
	var occurrences any
	if report.Occurrences != nil {
		occurrences = *report.Occurrences
	}
	observations := map[string]any{"Reporter": bugReporterValue(report.ReporterUserID), "Occurrences": occurrences}
	if !isTableOutput() {
		obj := make(map[string]any, len(fields)+1)
		for _, field := range fields {
			obj[field.Key] = field.Value
		}
		for key, value := range observations {
			obj[key] = value
		}
		if report.Environment == nil {
			obj["Environment"] = nil
		} else {
			obj["Environment"] = report.Environment
		}
		return formatter.FormatRaw(obj)
	}
	for _, key := range []string{"Reporter", "Occurrences"} {
		value := observations[key]
		text := "(not recorded)"
		if value != nil {
			text = fmt.Sprint(value)
			if number, ok := value.(float64); ok {
				text = strconv.FormatFloat(number, 'f', -1, 64)
			}
		}
		fields = append(fields, output.KeyValue{Key: key, Value: text})
	}
	environment := "(not recorded)"
	if report.Environment != nil {
		environment = output.NewFormatter("json").FormatRaw(report.Environment)
	}
	fields = append(fields, output.KeyValue{Key: "Environment", Value: environment})
	return formatter.FormatSingle(fields)
}

// encoding/json decodes interface numbers as float64. Preserve an internal
// reporter ID as an integer on YAML too, without coercing anonymous or null.
func bugReporterValue(value any) any {
	if number, ok := value.(float64); ok && number >= 1 && number <= math.MaxInt32 && number == math.Trunc(number) {
		return int64(number)
	}
	return value
}
