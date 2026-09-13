"""Run with Python + pytest and the same cached Go toolchain as go test ./... .

No production calls: Go tests use loopback httptest servers. Each behavior is
reverted only in a disposable copy; pytest must exit exactly1 with named
failures, while both low-level API controls stay green.
"""

import hashlib
import json
import shutil
import subprocess
import sys
import tempfile
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PREFIX = "scripts.red.test_board_cli::"
POST = PREFIX + "test_board_post_preserves_scope_identity_and_ttl"
READ = PREFIX + "test_board_read_quotes_untrusted_terminal_text"
CONTROLS = {PREFIX + "test_board_api_post_preserves_plain_data_control",
            PREFIX + "test_board_api_read_preserves_filters_control"}
REVERTS = [
    ("discard_qualified_owner", "internal/api/board.go",
     'params.Set("owner", owner)', '_ = owner', {POST}),
    ("misconvert_ttl", "cmd/board.go",
     'seconds := int64(duration / time.Second)',
     'seconds := int64(duration / time.Second) * 2', {POST}),
    ("render_body_as_terminal_commands", "cmd/board.go",
     'agent:%q (%q, %s): %q\\n', 'agent:%q (%q, %s): %s\\n', {READ}),
]


def run(repo, report, tests="scripts/red/test_board_cli.py"):
    result = subprocess.run(
        [sys.executable, "-m", "pytest", tests, "-q",
         "-p", "no:cacheprovider", f"--junitxml={report}"],
        cwd=repo, capture_output=True, text=True, timeout=120,
    )
    failed, passed, errors = set(), set(), set()
    assert report.exists(), result.stdout + result.stderr
    for test in ET.parse(report).iter("testcase"):
        name = test.get("classname") + "::" + test.get("name")
        if test.find("failure") is not None:
            failed.add(name)
        elif test.find("error") is not None or test.find("skipped") is not None:
            errors.add(name)
        else:
            passed.add(name)
    return result, failed, passed, errors


if __name__ == "__main__":
    originals = {path: (ROOT / path).read_bytes() for _, path, *_ in REVERTS}
    hashes = {path: hashlib.sha256(data).hexdigest() for path, data in originals.items()}
    with tempfile.TemporaryDirectory(prefix="astra-2158-cli-red-") as directory:
        repo = Path(directory) / "repo"
        shutil.copytree(ROOT, repo, ignore=shutil.ignore_patterns(
            ".git", ".worktrees", ".env*", "__pycache__", ".pytest_cache"))
        result, failed, passed, errors = run(repo, Path(directory) / "green.xml")
        assert result.returncode == 0 and not failed and not errors, result.stdout + result.stderr
        assert passed == CONTROLS | {POST, READ}, passed
        print("GREEN: four named tests", flush=True)
        for name, path, before, after, expected in REVERTS:
            source = originals[path].decode()
            assert source.count(before) == 1, name
            (repo / path).write_text(source.replace(before, after, 1))
            try:
                result, failed, passed, errors = run(repo, Path(directory) / f"{name}.xml")
                assert result.returncode == 1, result.stdout + result.stderr
                assert failed == expected and not errors, (failed, expected, errors, result.stdout)
                assert CONTROLS <= passed, passed
                print(json.dumps({"revert": name, "pytest_exit": 1, "failed": sorted(failed),
                                  "green_controls": sorted(CONTROLS)}), flush=True)
            finally:
                (repo / path).write_bytes(originals[path])
    assert all(hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == digest
               for path, digest in hashes.items())
    print("PASS: three decision reverts; original worktree unchanged")
