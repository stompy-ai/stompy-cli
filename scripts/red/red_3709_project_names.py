#!/usr/bin/env python3
"""Semantic reverts in disposable copies; originals and controls stay intact."""

import hashlib
import json
import shutil
import tempfile
from pathlib import Path

from red_2158_board_cli import run

ROOT = Path(__file__).resolve().parents[2]
PREFIX = "scripts.red.test_project_names::"
CONTROLS = {PREFIX + name for name in (
    "test_canonical_marker_control", "test_committed_pending_control",
    "test_owner_refusal_control", "test_arity_control",
    "test_name_endpoint_authentication_control", "test_existing_read_retry_control",
    "test_release_refusal_control", "test_aliases_owner_refusal_control",
)}
REVERTS = [
    ("omit_successful_alias_notice", "internal/api/project_notice.go",
     'envelope.NowCalled == ""', 'len(envelope.NowCalled) >= 0',
     {"test_notice_on_every_rest_command", "test_notice_quotes_untrusted_text"}),
    ("render_server_text_unquoted", "internal/api/project_notice.go",
     'Project is now called %q.', 'Project is now called %s.',
     {"test_notice_on_every_rest_command", "test_notice_quotes_untrusted_text"}),
    ("ignore_rename_marker_evidence", "cmd/project_check.go",
     'if resp.NowCalled != "" {', 'if false && resp.NowCalled != "" {',
     {"test_marker_correction_and_qualified_scope", "test_personal_marker_correction",
      "test_mixed_marker_failure_stays_failure", "test_unverifiable_marker_evidence_refused"}),
    ("strip_marker_owner", "cmd/project_name_address.go",
     'return response.NowCalled, nil', 'return current, nil',
     {"test_marker_correction_and_qualified_scope"}),
    ("accept_cross_owner_marker_evidence", "cmd/project_name_address.go",
     'formerOwner != currentOwner', 'false && formerOwner != currentOwner',
     {"test_unverifiable_marker_evidence_refused"}),
    ("omit_physical_readback_check", "cmd/project_rename.go",
     'current.SchemaName != result.SchemaName', 'false && current.SchemaName != result.SchemaName',
     {"test_recreated_name_cannot_verify", "test_release_changed_readback_refused"}),
    ("ignore_same_schema_recreation", "cmd/project_rename.go",
     'if !current.CreatedAt.Equal(before.CreatedAt) {',
     'if false && !current.CreatedAt.Equal(before.CreatedAt) {',
     {"test_recreated_schema_cannot_verify", "test_release_changed_readback_refused"}),
    ("accept_later_name_readback", "cmd/project_rename.go",
     'current.Name != result.Name || current.SchemaName != result.SchemaName || current.NowCalled != ""',
     'current.SchemaName != result.SchemaName',
     {"test_later_name_cannot_verify", "test_release_changed_readback_refused"}),
    ("accept_unverifiable_alias_list", "cmd/project_rename.go",
     'if aliases.Name != result.Name || aliases.SchemaName != result.SchemaName || aliases.Aliases == nil {',
     'if false && (aliases.Name != result.Name || aliases.SchemaName != result.SchemaName || aliases.Aliases == nil) {',
     {"test_live_alias_readback_required", "test_release_changed_readback_refused"}),
    ("drop_qualified_owner_on_readback", "cmd/project_rename.go",
     'if owner != "" {', 'if false && owner != "" {',
     {"test_qualified_rename_round_trip", "test_release_round_trip_and_binding"}),
    ("retry_ambiguous_alias_release", "internal/api/project_names.go",
     'nil, nil, false)', 'nil, nil, true)',
     {"test_release_must_not_retry"}),
    ("omit_release_confirmation", "cmd/project_release.go",
     'if !confirm {', 'if false && !confirm {',
     {"test_release_requires_confirmation"}),
    ("accept_still_live_released_name", "cmd/project_release.go",
     'if alias == args[0] {', 'if false && alias == args[0] {',
     {"test_release_retained_alias_refused"}),
    ("leave_aliases_unsorted", "cmd/project_aliases.go",
     'sort.Strings(result.Aliases)', 'sort.Strings([]string{})',
     {"test_aliases_current_sorted_and_empty"}),
    ("render_missing_alias_list_as_empty", "cmd/project_aliases.go",
     'result.Aliases == nil', 'false && result.Aliases == nil',
     {"test_aliases_unverifiable_list_refused"}),
]


def main():
    originals = {p: (ROOT / p).read_bytes() for _, p, *_ in REVERTS}
    hashes = {p: hashlib.sha256(data).hexdigest() for p, data in originals.items()}
    with tempfile.TemporaryDirectory(prefix="astra-3709-red-") as directory:
        copy = Path(directory) / "repo"
        shutil.copytree(ROOT, copy, ignore=shutil.ignore_patterns(
            ".git", ".worktrees", ".env*", "__pycache__", ".pytest_cache", "bin"))
        tests = "scripts/red/test_project_names.py"
        green, failures, passes, errors = run(copy, Path(directory) / "green.xml", tests)
        assert green.returncode == 0 and not failures and not errors, green.stdout + green.stderr
        universe = passes
        assert CONTROLS <= universe
        print(json.dumps({"green": sorted(universe)}), flush=True)
        for name, path, before, after, expected in REVERTS:
            text = originals[path].decode()
            assert text.count(before) == 1, (name, text.count(before))
            (copy / path).write_text(text.replace(before, after, 1))
            try:
                result, failures, passes, errors = run(copy, Path(directory) / (name + ".xml"), tests)
                expected = {PREFIX + test for test in expected}
                assert result.returncode == 1, result.stdout + result.stderr
                assert failures == expected and not errors, (name, failures, expected, errors, result.stdout)
                assert passes == universe - expected and CONTROLS <= passes, (name, passes)
                print(json.dumps({"revert": name, "pytest_exit": 1,
                                  "failed": sorted(failures), "controls": sorted(CONTROLS)}), flush=True)
            finally:
                (copy / path).write_bytes(originals[path])
    assert all(hashlib.sha256((ROOT / p).read_bytes()).hexdigest() == h for p, h in hashes.items())
    print("PASS: original worktree unchanged; exact failure/pass sets and collection retained")


if __name__ == "__main__":
    main()
