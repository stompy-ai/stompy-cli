"""Canonical filters stay composed; refusal and ordinary reporter controls stay green."""

from pathlib import Path

import red_3849_sessions as driver


if __name__ == "__main__":
    engine = driver.engine
    engine.ROOT = Path(__file__).resolve().parents[2]
    engine.TEST = "scripts/red/test_bug_filters_3865.py"
    engine.PREFIX = "scripts.red.test_bug_filters_3865::"
    case = lambda label: "test_real_command_tree[" + label + "]"
    engine.CONTROLS = {engine.PREFIX + case(label)
                       for label in ("unfiltered", "auth_help", "refusal")}
    filters = {case(label) for label in ("critical", "high", "medium", "low")}
    engine.REVERTS = [
        ("discard_severity_filter", "internal/api/bugs.go",
         'params.Set("severity", severity)', '_ = severity', filters),
        ("discard_composed_status", "internal/api/bugs.go",
         'params.Set("status", status)', '_ = status', filters),
        ("discard_composed_limit", "internal/api/bugs.go",
         'params.Set("limit", strconv.Itoa(limit))', '_ = strconv.Itoa(limit)', filters),
        ("discard_composed_offset", "internal/api/bugs.go",
         'params.Set("offset", strconv.Itoa(offset))', '_ = strconv.Itoa(offset)', filters),
        ("restore_stale_status_help", "cmd/bug.go",
         'Filter by status (triage, confirmed, in_progress, resolved, wont_fix, parked)',
         'Filter by status (new, triaged, in_progress, resolved, closed, duplicate)',
         {case("help")}),
    ]
    driver.run_reverts()
