"""Exact real command-tree cases; unknown flags fail as behavior, not compilation."""

import json
import re
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
CASES = {severity: "TestBug3865_SeverityComposesWithCanonicalFilters/" + severity
         for severity in ("critical", "high", "medium", "low")}
CASES.update(help="TestBug3865_HelpNamesCanonicalStatuses",
             unfiltered="TestBug3865_UnfilteredControl",
             auth_help="TestBug3865_ReporterAuthHelpControl",
             refusal="TestBug3865_RefusalControl")


@pytest.fixture(scope="session")
def binary(tmp_path_factory):
    binary = tmp_path_factory.mktemp("bug-filters") / "cmd.test"
    subprocess.run(["go", "test", "-c", "-o", str(binary), "./cmd"],
                   cwd=ROOT, check=True, timeout=120)
    return binary


@pytest.mark.parametrize("case", CASES)
def test_real_command_tree(binary, case):
    name = CASES[case]
    result = subprocess.run(
        ["go", "tool", "test2json", "-p", "cmd", str(binary), "-test.v",
         "-test.run", "/".join("^" + re.escape(p) + "$" for p in name.split("/")),
         "-test.timeout=10s"], cwd=ROOT, capture_output=True, text=True, timeout=15)
    rows = [json.loads(line) for line in result.stdout.splitlines()]
    verdicts = {r["Test"]: r["Action"] for r in rows
                if r.get("Test") and r["Action"] in {"pass", "fail", "skip"}}
    leaves = {n: v for n, v in verdicts.items()
              if not any(other.startswith(n + "/") for other in verdicts)}
    assert set(leaves) == {name}, result.stdout + result.stderr
    assert result.returncode == 0 and leaves[name] == "pass", result.stdout + result.stderr
