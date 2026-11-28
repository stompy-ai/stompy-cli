"""Drive real Cobra commands; pytest RED names each semantic projection loss."""

from __future__ import annotations

import json
import re
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
CASES = {a + "-" + f: "TestBug3931_ReporterAndOccurrences/" + a + "/" + f
         for a in ("owned", "anonymous") for f in ("json", "yaml", "table")}
CASES.update({v + "-" + f: "TestBug3931_MissingObservationIsNotInvented/" + v + "/" + f
              for v in ("absent", "null") for f in ("json", "yaml", "table")})
CASES.update({"list-" + f: "TestBug3931_ListContext/" + f for f in ("json", "yaml", "table")})
CASES.update(refusal="TestBug3931_RefusalControl",
             context_control="TestBug3867_CanonicalContextFields/json",
             quarantine_json="TestBugListNamesQuarantinedRowsInJSONOutput3848",
             quarantine_table="TestBugListNamesQuarantinedRowsInTableOutput3848",
             no_project_list="TestBugListNeedsNoProject3848",
             no_project_get="TestBugGetNeedsNoProject3848")


@pytest.fixture(scope="session")
def binary(tmp_path_factory: pytest.TempPathFactory) -> Path:
    binary = tmp_path_factory.mktemp("fields3931") / "cmd.test"
    subprocess.run(["go", "test", "-c", "-o", str(binary), "./cmd"],
                   cwd=ROOT, check=True, timeout=120)
    return binary


@pytest.mark.parametrize("case", CASES)
def test_real_command_tree(binary: Path, case: str) -> None:
    name = CASES[case]
    result = subprocess.run([
        "go", "tool", "test2json", "-p", "cmd", str(binary), "-test.v", "-test.run",
        "/".join("^" + re.escape(p) + "$" for p in name.split("/")), "-test.timeout=10s"],
        cwd=ROOT, capture_output=True, text=True, timeout=15)
    rows = [json.loads(line) for line in result.stdout.splitlines()]
    verdicts = {r["Test"]: r["Action"] for r in rows
                if r.get("Test") and r["Action"] in {"pass", "fail", "skip"}}
    leaves = {n: v for n, v in verdicts.items()
              if not any(other.startswith(n + "/") for other in verdicts)}
    assert set(leaves) == {name}, result.stdout + result.stderr
    assert result.returncode == 0 and leaves[name] == "pass", result.stdout + result.stderr
