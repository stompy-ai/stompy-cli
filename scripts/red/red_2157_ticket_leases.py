"""Lease transport decision reverts, exact pytest identities and green close."""

import hashlib
import shutil
import tempfile
from pathlib import Path

from red_2158_board_cli import run

ROOT = Path(__file__).resolve().parents[2]
TESTS = "scripts/red/test_ticket_leases.py"
PREFIX = "scripts.red.test_ticket_leases::"
CALL = PREFIX + "test_lease_actions_keep_owner_label_and_single_write"
REFUSE = PREFIX + "test_lease_refusal_is_not_success"
CONTROL = PREFIX + "test_canonical_close_remains_available_control"
REVERTS = [
    ("discard_qualified_owner", "internal/api/ticket_leases.go", 'params.Set("owner", owner)', '_ = owner', {CALL}),
    ("discard_agent_label", "cmd/ticket_lease.go", "AgentLabel: label", 'AgentLabel: "" + label[:0]', {CALL}),
    ("swallow_held_refusal", "internal/api/ticket_leases.go",
     "c.Post(path, request, &response); err != nil {", "c.Post(path, request, &response); err != nil && false {", {REFUSE}),
]


if __name__ == "__main__":
    originals = {p: (ROOT / p).read_bytes() for _, p, *_ in REVERTS}
    hashes = {p: hashlib.sha256(b).hexdigest() for p, b in originals.items()}
    with tempfile.TemporaryDirectory(prefix="astra-2157-cli-red-") as directory:
        repo = Path(directory) / "repo"
        shutil.copytree(ROOT, repo, ignore=shutil.ignore_patterns(".git", ".worktrees", ".env*", "__pycache__", ".pytest_cache"))
        result, failed, passed, errors = run(repo, Path(directory) / "green.xml", TESTS)
        assert result.returncode == 0 and not failed and not errors, result.stdout + result.stderr
        assert passed == {CALL, REFUSE, CONTROL}, passed
        print("GREEN: three named tests", flush=True)
        for name, path, before, after, expected in REVERTS:
            source = originals[path].decode()
            assert source.count(before) == 1, name
            (repo / path).write_text(source.replace(before, after, 1))
            try:
                result, failed, passed, errors = run(repo, Path(directory) / f"{name}.xml", TESTS)
                assert result.returncode == 1 and failed == expected and not errors, (result.stdout, result.stderr, failed, errors)
                assert CONTROL in passed
                print(f"RED {name}: pytest exactly1; failures={sorted(failed)}; GREEN control={CONTROL}", flush=True)
            finally:
                (repo / path).write_bytes(originals[path])
    assert all(hashlib.sha256((ROOT / p).read_bytes()).hexdigest() == h for p, h in hashes.items())
    print("PASS: three named reverts; original source unchanged")
