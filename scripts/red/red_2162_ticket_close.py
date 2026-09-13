"""Execute the direct-transition regression in an isolated source copy."""

import hashlib
import shutil
import tempfile
from pathlib import Path

from red_2158_board_cli import run

ROOT = Path(__file__).resolve().parents[2]
TESTS = "scripts/red/test_ticket_close.py"
FAILURE = "scripts.red.test_ticket_close::test_close_uses_server_walk_and_preserves_refusal"
CONTROL = "scripts.red.test_ticket_close::test_explicit_transition_remains_available_control"


if __name__ == "__main__":
    source = ROOT / "cmd/ticket.go"
    original = source.read_bytes()
    digest = hashlib.sha256(original).hexdigest()
    with tempfile.TemporaryDirectory(prefix="astra-2162-cli-red-") as directory:
        repo = Path(directory) / "repo"
        shutil.copytree(ROOT, repo, ignore=shutil.ignore_patterns(
            ".git", ".worktrees", ".env*", "__pycache__", ".pytest_cache"))
        result, failed, passed, errors = run(repo, Path(directory) / "green.xml", TESTS)
        assert result.returncode == 0 and not failed and not errors, result.stdout
        assert passed == {FAILURE, CONTROL}, passed
        before = "apiClient.CloseTicket(project, id)"
        after = 'apiClient.TransitionTicket(project, id, "done")'
        text = original.decode()
        assert text.count(before) == 1
        (repo / "cmd/ticket.go").write_text(text.replace(before, after, 1))
        result, failed, passed, errors = run(repo, Path(directory) / "red.xml", TESTS)
        assert result.returncode == 1, result.stdout + result.stderr
        assert failed == {FAILURE} and not errors, (failed, errors, result.stdout)
        assert passed == {CONTROL}, passed
        print("GREEN: two named tests; RED direct_transition: pytest exit exactly 1")
        print("failed:", FAILURE)
        print("GREEN control:", CONTROL)
    assert hashlib.sha256(source.read_bytes()).hexdigest() == digest
    print("PASS: original worktree unchanged")
