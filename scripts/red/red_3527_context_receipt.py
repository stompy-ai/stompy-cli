#!/usr/bin/env python3
"""Remove only the human receipt-date decision; preserve the request and JSON."""

import hashlib
import shutil
import tempfile
from pathlib import Path

from red_2158_board_cli import run

ROOT = Path(__file__).resolve().parents[2]
TESTS = "scripts/red/test_context_receipt.py"
MODULE = "scripts.red.test_context_receipt::"
FAILURE = MODULE + "test_cli_replay_shows_the_receipt_deadline"
CONTROLS = {MODULE + name for name in ("test_cli_legacy_human_output_control",
                                     "test_cli_json_response_control")}


if __name__ == "__main__":
    source = ROOT / "cmd/context_lifecycle.go"
    original = source.read_bytes()
    digest = hashlib.sha256(original).hexdigest()
    with tempfile.TemporaryDirectory(prefix="astra-3527-cli-red-") as directory:
        repo = Path(directory) / "repo"
        shutil.copytree(ROOT, repo, ignore=shutil.ignore_patterns(
            ".git", ".worktrees", ".env*", "__pycache__", ".pytest_cache", "artifacts"))
        result, failed, passed, errors = run(repo, Path(directory) / "green.xml", TESTS)
        assert result.returncode == 0 and not failed and not errors, result.stdout + result.stderr
        assert passed == {FAILURE} | CONTROLS, passed
        before = '\t\tif r.ReviewAfter != "" {\n'
        assert original.decode().count(before) == 1
        (repo / "cmd/context_lifecycle.go").write_text(original.decode().replace(
            before, '\t\tif false && r.ReviewAfter != "" {\n', 1))
        result, failed, passed, errors = run(repo, Path(directory) / "red.xml", TESTS)
        assert result.returncode == 1, result.stdout + result.stderr
        assert failed == {FAILURE} and passed == CONTROLS and not errors, (failed, passed, errors)
        print("pytest exit exactly 1; failed:", sorted(failed), "GREEN:", sorted(passed))
    assert hashlib.sha256(source.read_bytes()).hexdigest() == digest
    print("PASS original source unchanged; exact test universe retained")
