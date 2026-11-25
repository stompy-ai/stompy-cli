"""Replay actual MCP-created reports through REST and the compiled candidate CLI.

From a backend worktree: LOCAL_CI_PORT_SHIFT=200 python3 scripts/local_ci.py run --
python /path/to/cli/scripts/red/prove_bug_fields_3867.py --binary /path/to/stompy --out /tmp/proof
All databases belong to the existing disposable fixture; only auth is synthetic.
"""

import argparse
import asyncio
import json
import os
import subprocess
import sys
from pathlib import Path

import pytest
import yaml

records = []


async def readback(case, tmp_path, binary):
    from fastapi.testclient import TestClient
    from tests.conftest import parse_tool_response
    from tests.integration.fixtures_2210 import one
    from tests.integration.fixtures_3705_doors import running
    from tests.integration.fixtures_3834_cutover import counts, seed

    public_id = "BUG-" + str(one(case.w.main, "SELECT id FROM bug_inbox.tickets"))
    foreign = seed(case, reporter=case.w.owner_id, title="foreign context")
    metadata = json.loads(one(case.w.main, "SELECT metadata FROM bug_inbox.tickets WHERE id=%s",
                              (int(foreign["id"][4:]),)))
    assert metadata["reporter_user_id"] == case.w.owner_id
    before = counts(case)
    assert before == (2, 0, 0, 0)
    try:
        row = parse_tool_response(await case.host.bug_report_get(report_id=public_id))
        assert row["id"] == public_id and row["environment"]
        assert row["session_id"] and row["updated_at"]
        refused = parse_tool_response(await case.host.bug_report_get(report_id=foreign["id"]))
        assert refused["error"] == "NOT_FOUND"
        with TestClient(case.app) as client:
            rest = client.get("/api/v1/bug-reports/" + public_id)
            assert rest.status_code == 200 and rest.json() == row
            assert client.get("/api/v1/bug-reports/" + foreign["id"]).status_code == 404
        want = {"ID": public_id, "Status": row["status"], "Severity": row["severity"],
                "Project Name": row["project_name"] or "", "Session ID": row["session_id"],
                "Environment": row["environment"], "Updated": row["updated_at"]}
        with running(case.app) as url:
            command = [str(binary), "--api-url", url, "--api-key", "synthetic"]
            env = {"PATH": "/usr/bin:/bin", "HOME": str(tmp_path), "NO_COLOR": "1"}
            for format in ("json", "yaml", "table"):
                result = subprocess.run(command + ["-o", format, "bug", "get", public_id],
                                        cwd=tmp_path, env=env, capture_output=True,
                                        text=True, timeout=20)
                assert result.returncode == 0, result.stderr
                if format != "table":
                    actual = json.loads(result.stdout) if format == "json" else yaml.safe_load(result.stdout)
                    assert {key: actual[key] for key in want} == want
                else:
                    for key in ("Project Name", "Session ID", "Environment", "Updated"):
                        assert key in result.stdout
                    for value in (public_id, row["status"], row["severity"], row["session_id"],
                                  row["updated_at"], row["project_name"] or "(not recorded)"):
                        assert value in result.stdout
                    assert '"tool": "bug_report"' in result.stdout
            hidden = subprocess.run(command + ["-o", "json", "bug", "get", foreign["id"]],
                                    cwd=tmp_path, env=env, capture_output=True,
                                    text=True, timeout=20)
            assert hidden.returncode != 0 and "404" in hidden.stderr and not hidden.stdout
        records.append(dict(report=row, expected_cli_fields=want, formats=["json", "yaml", "table"],
                            actual_mcp_create=True, rest_equal_mcp=True, compiled_cli=True,
                            foreign_refused_all_three=True, physical_counts=before))
    finally:
        assert counts(case) == before


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    assert os.environ.get("TEST_DATABASE_URL"), "use backend local_ci run"
    assert (Path.cwd() / "tests/integration/fixtures_3834_cutover.py").exists()
    sys.path.insert(0, str(Path.cwd()))
    args.out.mkdir(parents=True, exist_ok=True)
    class Oracle:
        def pytest_collection_modifyitems(self, items):
            for item in items:
                if "tmp_path" not in item.fixturenames:
                    item.fixturenames.append("tmp_path")

        @pytest.hookimpl(wrapper=True)
        def pytest_runtest_call(self, item):
            result = yield
            asyncio.run(readback(item.funcargs["doors"], item.funcargs["tmp_path"], args.binary.resolve()))
            return result
    code = pytest.main([
        "tests/integration/test_3834_cutover_doors.py::test_submit_has_one_main_target_with_or_without_project",
        "-v", "-p", "no:cacheprovider", "--junitxml=" + str(args.out / "native.xml"),
    ], plugins=[Oracle()])
    (args.out / "readback.json").write_text(json.dumps(records, indent=2) + "\n")
    if code == 0:
        assert len(records) == 4
    return code


if __name__ == "__main__":
    raise SystemExit(main())
