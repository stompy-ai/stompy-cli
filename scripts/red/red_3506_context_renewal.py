"""Discard only the renewal-key decision; keep the flag and API signatures."""

import hashlib
import shutil
import tempfile
from pathlib import Path

from red_2158_board_cli import run

ROOT = Path(__file__).resolve().parents[2]
TESTS = "scripts/red/test_context_renewal.py"
FAILURE = "scripts.red.test_context_renewal::test_cli_keeps_the_callers_renewal_key_on_retries"
CONTROL = "scripts.red.test_context_renewal::test_cli_without_renewal_keeps_the_legacy_request_control"


if __name__ == "__main__":
    source = ROOT / "cmd/context_lifecycle.go"
    original = source.read_bytes()
    digest = hashlib.sha256(original).hexdigest()
    with tempfile.TemporaryDirectory(prefix="astra-3506-cli-red-") as directory:
        repo = Path(directory) / "repo"
        shutil.copytree(ROOT, repo, ignore=shutil.ignore_patterns(
            ".git", ".worktrees", ".env*", "__pycache__", ".pytest_cache"))
        result, failed, passed, errors = run(repo, Path(directory) / "green.xml", TESTS)
        assert result.returncode == 0 and not failed and not errors, result.stdout
        assert passed == {FAILURE, CONTROL}, passed
        before = 'req.RenewalKey, _ = cmd.Flags().GetString("renewal-key")'
        after = 'req.RenewalKey = ""'
        text = original.decode()
        assert text.count(before) == 1
        (repo / "cmd/context_lifecycle.go").write_text(text.replace(before, after, 1))
        result, failed, passed, errors = run(repo, Path(directory) / "red.xml", TESTS)
        assert result.returncode == 1, result.stdout + result.stderr
        assert failed == {FAILURE} and not errors, (failed, errors, result.stdout)
        assert passed == {CONTROL}, passed
        print("RED discard_renewal_key: pytest exit exactly 1")
        print("failed:", FAILURE)
        print("GREEN control:", CONTROL)
    assert hashlib.sha256(source.read_bytes()).hexdigest() == digest
    print("PASS: original worktree unchanged")
