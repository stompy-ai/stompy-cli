"""Compiled format decisions: exact RED identities and invariant controls."""

import hashlib
import json
import shutil
import tempfile
from pathlib import Path

from red_2158_board_cli import run

ROOT = Path(__file__).resolve().parents[2]
TEST = "scripts/red/test_output_formats_3738.py"
PREFIX = "scripts.red.test_output_formats_3738::"
JSON = "test_compiled_check_normalizes_json_and_preserves_qualified_binding"
CONFIG = "test_configured_format_is_normalized_and_validated"
CONTROLS = {
    PREFIX + name
    for name in (
        "test_lowercase_json_mixed_failure_control",
        "test_canonical_json_control_has_no_rename_or_marker_edit",
    )
}
REVERTS = [
    (
        "skip_case_normalization",
        "cmd/output_format.go",
        "return strings.ToLower(strings.TrimSpace(rawOutputFormat()))",
        "return strings.TrimSpace(rawOutputFormat())",
        {CONFIG}
        | {
            JSON + f"[{suffix}-{value}]"
            for suffix in ("False", "True")
            for value in ("JSON", "Json")
        },
    ),
    (
        "skip_whitespace_normalization",
        "cmd/output_format.go",
        "return strings.ToLower(strings.TrimSpace(rawOutputFormat()))",
        "return strings.ToLower(rawOutputFormat())",
        {JSON + f"[{suffix}- json ]" for suffix in ("False", "True")},
    ),
    (
        "admit_unknown_format",
        "cmd/output_format.go",
        'return &exitError{2, fmt.Sprintf("unknown output format %q: use table, json or yaml", rawOutputFormat())}',
        'if false { return &exitError{2, fmt.Sprintf("unknown output format %q: use table, json or yaml", rawOutputFormat())} }; return nil',
        {CONFIG}
        | {f"test_unknown_format_is_exit_two_before_auth_or_network[command{i}]" for i in range(3)},
    ),
    (
        "discard_correction_owner",
        "cmd/project_name_address.go",
        "return response.NowCalled, nil",
        "return current, nil",
        {"test_table_control_preserves_actionable_correction"}
        | {
            JSON + f"[{suffix}-{value}]"
            for suffix in ("False", "True")
            for value in ("JSON", "Json", " json ")
        },
    ),
]


def main():
    originals = {path: (ROOT / path).read_bytes() for _, path, *_ in REVERTS}
    hashes = {path: hashlib.sha256(data).hexdigest() for path, data in originals.items()}
    with tempfile.TemporaryDirectory(prefix="red-3738-cli-") as directory:
        repo = Path(directory) / "repo"
        shutil.copytree(
            ROOT,
            repo,
            ignore=shutil.ignore_patterns(
                ".git", ".venv*", ".worktrees", ".claude", "__pycache__", ".pytest_cache", "bin"
            ),
        )
        result, failed, passed, errors = run(repo, Path(directory) / "green.xml", TEST)
        assert result.returncode == 0 and not failed and not errors, result.stdout + result.stderr
        universe = passed
        assert CONTROLS <= universe
        print(f"GREEN: {len(universe)} identities, {len(CONTROLS)} controls", flush=True)
        for name, path, before, after, expected in REVERTS:
            source = originals[path].decode()
            assert source.count(before) == 1, name
            (repo / path).write_text(source.replace(before, after, 1))
            try:
                result, failed, passed, errors = run(repo, Path(directory) / f"{name}.xml", TEST)
                expected = {PREFIX + item for item in expected}
                assert result.returncode == 1 and not errors, result.stdout + result.stderr
                assert failed == expected, (name, failed, expected, result.stdout)
                assert passed == universe - expected and CONTROLS <= passed
                print(
                    json.dumps(
                        {
                            "revert": name,
                            "pytest_exit": 1,
                            "failed": sorted(failed),
                            "controls": sorted(CONTROLS),
                        }
                    ),
                    flush=True,
                )
            finally:
                (repo / path).write_bytes(originals[path])
    assert all(
        hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == value
        for path, value in hashes.items()
    )
    print("PASS: four exact named reverts; original worktree unchanged")


if __name__ == "__main__":
    main()
