"""Exercise context projection and retain #70's five quarantine/project guards."""

import json
import re
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
CASES = {"present-" + f: "TestBug3867_CanonicalContextFields/" + f
         for f in ("json", "yaml", "table")}
CASES.update({v + "-" + f: "TestBug3867_AbsentContextPolicy/" + v + "/" + f
              for v in ("absent", "null", "empty_object")
              for f in ("json", "yaml", "table")})
CASES.update(refusal="TestBug3867_RefusalControl",
             quarantine_table="TestBugListNamesQuarantinedRowsInTableOutput3848",
             quarantine_json="TestBugListNamesQuarantinedRowsInJSONOutput3848",
             clean_list="TestControlBugListWithoutQuarantineSaysNothingExtra3848",
             no_project_list="TestBugListNeedsNoProject3848",
             no_project_get="TestBugGetNeedsNoProject3848")


@pytest.fixture(scope="session")
def binary(tmp_path_factory):
    binary = tmp_path_factory.mktemp("bug-fields") / "cmd.test"
    subprocess.run(["go", "test", "-c", "-o", str(binary), "./cmd"],
                   cwd=ROOT, check=True, timeout=120)
    return binary


@pytest.mark.parametrize("case", CASES)
def test_real_command_tree(binary, case):
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
