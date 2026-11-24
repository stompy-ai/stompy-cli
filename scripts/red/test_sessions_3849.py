"""Pytest names for the real Cobra/httptest Go cases; no command mocks."""

import json
import re
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
CASES = {}
for environment in ("production", "staging"):
    for label, test in {
        "refresh_error": "TestSession3849_RefreshFailureNamesEnvironment",
        "whoami_no_refresh": "TestSession2562_WhoamiExpiredWithoutRefreshNeedsLogin",
        "whoami_with_refresh": "TestSession2562_WhoamiExpiredWithRefreshIsConditional",
        "refresh_success": "TestSession3849_RefreshSuccessSavesSelectedEnvironment",
        "valid_oauth": "TestSession3849_ValidOAuthSkipsRefresh",
        "no_session": "TestSession3849_NoSessionKeepsGenericError",
        "whoami_valid": "TestSession2562_WhoamiValidStaysValid",
    }.items():
        CASES[f"{label}-{environment}"] = f"{test}/{environment}"
    for cause in ("bad_json", "network"):
        CASES[f"cause_{cause}-{environment}"] = (
            f"TestSession3849_RefreshFailurePreservesCause/{environment}/{cause}"
        )
    for source in ("flag", "environment", "config"):
        CASES[f"fallback_{source}-{environment}"] = (
            f"TestSession3849_FallbackStillAuthenticates/{environment}/{source}"
        )


@pytest.fixture(scope="session")
def binary(tmp_path_factory):
    path = tmp_path_factory.mktemp("session-cli") / "cmd.test"
    subprocess.run(
        ["go", "test", "-c", "-o", str(path), "./cmd"], cwd=ROOT, check=True, timeout=120
    )
    return path


@pytest.mark.parametrize("case", CASES)
def test_real_command_tree(binary, case):
    name = CASES[case]
    expression = "/".join("^" + re.escape(part) + "$" for part in name.split("/"))
    result = subprocess.run(
        ["go", "tool", "test2json", "-p", "cmd", str(binary), "-test.v",
         "-test.run", expression, "-test.timeout=20s"],
        cwd=ROOT, capture_output=True, text=True, timeout=30,
    )
    rows = [json.loads(line) for line in result.stdout.splitlines()]
    verdicts = {
        row["Test"]: row["Action"] for row in rows
        if row.get("Test") and row["Action"] in {"pass", "fail", "skip"}
    }
    leaves = {
        test: verdict for test, verdict in verdicts.items()
        if not any(other.startswith(test + "/") for other in verdicts)
    }
    assert set(leaves) == {name}, result.stdout + result.stderr
    assert result.returncode == 0 and leaves[name] == "pass", result.stdout + result.stderr
