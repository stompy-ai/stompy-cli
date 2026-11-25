"""Context decisions, exact named RED; original #70 controls stay green."""

from pathlib import Path

import red_3849_sessions as driver


if __name__ == "__main__":
    engine = driver.engine
    engine.ROOT = Path(__file__).resolve().parents[2]
    engine.TEST = "scripts/red/test_bug_fields_3867.py"
    engine.PREFIX = "scripts.red.test_bug_fields_3867::"
    case = lambda label: "test_real_command_tree[" + label + "]"
    engine.CONTROLS = {engine.PREFIX + case(label) for label in (
        "refusal", "quarantine_table", "quarantine_json", "clean_list",
        "no_project_list", "no_project_get")}
    present = {case("present-" + f) for f in ("json", "yaml", "table")}
    helper = "cmd/bug_detail.go"
    engine.REVERTS = [
        ("discard_project_context", helper,
         'Value: report.ProjectName', 'Value: ""', present),
        ("discard_session_context", helper,
         'Value: report.SessionID', 'Value: ""', present),
        ("discard_updated_timestamp", helper,
         'Value: report.UpdatedAt', 'Value: ""', present),
        ("discard_machine_environment", helper,
         'obj["Environment"] = report.Environment', 'obj["Environment"] = nil',
         {case(v + "-" + f) for v in ("present", "empty_object")
          for f in ("json", "yaml")}),
        ("discard_human_environment", helper,
         'if report.Environment != nil {', 'if false {',
         {case(v + "-table") for v in ("present", "empty_object")}),
    ]
    driver.run_reverts()
