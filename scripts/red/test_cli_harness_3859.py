"""Run the real Cobra harness cases; the Go timeout test owns a bounded child."""

import json
import re
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
CASES = {
    "unknown_flag": "TestCLI3859_UnknownFlagReturnsPromptly",
    "omitted_context_list": "TestCLI3859_ContextListFlagsDoNotLeak",
    "deep_local_and_persistent": "TestCLI3859_DeepLocalAndPersistentFlagsDoNotLeak",
    "slice_flags": "TestCLI3859_SliceFlagsDoNotLeak",
    "control_search_json": "TestSearchRoutes_JSONIsTheAPIObject/-o_json_context_search_slimy",
    "control_server_error": "TestSearchRoutes_ServerErrorIsAnErrorNotAnEmptyList/search_q",
}


@pytest.fixture(scope="session")
def binary(tmp_path_factory):
    path = tmp_path_factory.mktemp("lifecycle-harness") / "cmd.test"
    subprocess.run(["go", "test", "-c", "-o", str(path), "./cmd"],
                   cwd=ROOT, check=True, timeout=120)
    return path


@pytest.mark.parametrize("case", CASES)
def test_real_harness(binary, case):
    name = CASES[case]
    result = subprocess.run(
        ["go", "tool", "test2json", "-p", "cmd", str(binary), "-test.v",
         "-test.run", "/".join("^" + re.escape(part) + "$" for part in name.split("/")), "-test.timeout=10s"],
        cwd=ROOT, capture_output=True, text=True, timeout=15,
    )
    rows = [json.loads(line) for line in result.stdout.splitlines()]
    verdicts = {
        row["Test"]: row["Action"] for row in rows
        if row.get("Test") and row["Action"] in {"pass", "fail", "skip"}
    }
    leaves = {test: verdict for test, verdict in verdicts.items()
              if not any(other.startswith(test + "/") for other in verdicts)}
    assert set(leaves) == {name}, result.stdout + result.stderr
    assert result.returncode == 0 and leaves[name] == "pass", result.stdout + result.stderr
