"""Canonical reader fields stay typed and visible; old reader controls stay green."""

from pathlib import Path

import red_3849_sessions as driver


if __name__ == "__main__":
    engine = driver.engine
    engine.ROOT = Path(__file__).resolve().parents[2]
    engine.TEST = "scripts/red/test_bug_fields_3931.py"
    engine.PREFIX = "scripts.red.test_bug_fields_3931::"
    case = lambda label: "test_real_command_tree[" + label + "]"
    engine.CONTROLS = {engine.PREFIX + case(label) for label in (
        "refusal", "context_control", "quarantine_json", "quarantine_table",
        "no_project_list", "no_project_get")}
    present = {case(a + "-" + f) for a in ("owned", "anonymous")
               for f in ("json", "yaml", "table")}
    listed = {case("list-" + f) for f in ("json", "yaml", "table")}
    detail = "cmd/bug_detail.go"
    engine.REVERTS = [
        ("discard_reporter_at_decode", "internal/api/bugs.go",
         '`json:"reporter_user_id"`', '`json:"red_unused_reporter"`', present),
        ("discard_occurrences_at_decode", "internal/api/bugs.go",
         '`json:"occurrences"`', '`json:"red_unused_occurrences"`', present),
        ("discard_reporter_at_render", detail,
         '"Reporter": bugReporterValue(report.ReporterUserID)', '"Reporter": nil', present),
        ("discard_occurrences_at_render", detail,
         '"Occurrences": occurrences',
         '"Occurrences": func(value any) any { return nil }(occurrences)', present),
        ("discard_project_on_list", "cmd/bug.go", 'b.ProjectName,', '"",', listed),
        ("discard_updated_on_list", "cmd/bug.go", 'b.UpdatedAt,', '"",', listed),
        ("omit_unknown_machine_observations", detail,
         'obj[key] = value', 'if value != nil { obj[key] = value }',
         {case(v + "-" + f) for v in ("absent", "null") for f in ("json", "yaml")}),
        ("hide_unknown_human_observations", detail,
         'text := "(not recorded)"', 'text := ""',
         {case(v + "-table") for v in ("absent", "null")}),
    ]
    driver.run_reverts()
