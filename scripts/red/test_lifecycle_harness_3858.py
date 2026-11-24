"""Run the real Cobra harness cases; the Go timeout test owns a bounded child."""

import json
import re
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
CASES = {
    "unknown_flag": "TestLifecycle3858_UnknownFlagReturnsPromptly",
    "invalid_flag_value": "TestLifecycle3858_InvalidFlagValueReturnsPromptly",
    "early_pre_run_error": "TestLifecycle3858_PreRunErrorReturnsPromptly",
    "top_level_flags": "TestLifecycle3858_TopLevelSearchFlagsDoNotLeak",
    "nested_flags": "TestLifecycle3858_NestedSearchFlagsDoNotLeak",
    "deep_local_and_persistent": "TestLifecycle3858_DeepLocalAndPersistentFlagsDoNotLeak",
    "slice_flags": "TestLifecycle3858_SliceFlagsDoNotLeak",
    "control_lifecycle_readback": "TestContextLifecycle_JSONIsTheAPIsObject",
    "control_command_registration": "TestContextLifecycle_IsRegisteredUnderContext",
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
         "-test.run", "^" + re.escape(name) + "$", "-test.timeout=10s"],
        cwd=ROOT, capture_output=True, text=True, timeout=15,
    )
    rows = [json.loads(line) for line in result.stdout.splitlines()]
    verdicts = {
        row["Test"]: row["Action"] for row in rows
        if row.get("Test") and row["Action"] in {"pass", "fail", "skip"}
    }
    assert set(verdicts) == {name}, result.stdout + result.stderr
    assert result.returncode == 0 and verdicts[name] == "pass", result.stdout + result.stderr
