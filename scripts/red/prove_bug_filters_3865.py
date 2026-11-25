"""Extend the existing native severity oracle with the compiled candidate CLI.

Run from a backend worktree through LOCAL_CI_PORT_SHIFT=200 local_ci run:
python /path/to/cli/scripts/red/prove_bug_filters_3865.py --binary /path/to/stompy --out /tmp/proof
The existing fixture owns and drops all databases; only its auth is synthetic.
"""

import argparse
import asyncio
import json
import os
import subprocess
import sys
from pathlib import Path

import pytest

records = []


async def readback(case, severity, tmp_path, binary):
    from fastapi.testclient import TestClient
    from tests.conftest import parse_tool_response
    from tests.integration.fixtures_3705_doors import running
    from tests.integration.fixtures_3834_cutover import counts, seed

    seed(case, reporter=case.w.owner_id, severity=severity, title="another reporter")
    before = counts(case)
    assert before == (3, 0, 0, 0)
    try:
        mcp = parse_tool_response(await case.host.bug_report_list(severity=severity))
        assert mcp["total"] == 1
        report = mcp["reports"][0]
        assert report["severity"] == severity
        with TestClient(case.app) as client:
            rest = client.get("/api/v1/bug-reports", params={"severity": severity})
            assert rest.status_code == 200
            assert rest.json()["reports"] == mcp["reports"]
            assert rest.json()["total"] == 1
        with running(case.app) as url:
            command = [str(binary), "--api-url", url, "--api-key", "synthetic",
                       "-p", "foo", "bug", "list", "--severity", severity]
            env = {"PATH": "/usr/bin:/bin", "HOME": str(tmp_path), "NO_COLOR": "1"}
            def run(*args):
                result = subprocess.run(command + list(args), cwd=tmp_path, env=env,
                                        capture_output=True, text=True, timeout=20)
                assert result.returncode == 0, result.stderr
                return result.stdout
            rows = json.loads(run("-o", "json"))
            assert [r["ID"] for r in rows] == [report["id"]]
            assert rows[0]["SEVERITY"] == severity
            table = run()
            assert report["id"] in table and "Total: 1 bug reports" in table
            assert json.loads(run("-o", "json", "--limit", "1", "--offset", "1")) == []
            assert "Total: 1 bug reports" in run("--limit", "1", "--offset", "1")
        records.append(dict(severity=severity, report_id=report["id"], total=1,
                            mcp=True, rest=True, compiled_cli=True,
                            foreign_reporter_excluded=True, empty_page_total=1,
                            physical_counts=before))
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
            asyncio.run(readback(item.funcargs["doors"], item.funcargs["severity"],
                                 item.funcargs["tmp_path"], args.binary.resolve()))
            return result
    code = pytest.main([
        "tests/integration/test_3834_cutover_doors.py::test_reader_shape_and_severity_are_identical",
        "-v", "-p", "no:cacheprovider", "--junitxml=" + str(args.out / "native.xml"),
    ], plugins=[Oracle()])
    (args.out / "readback.json").write_text(json.dumps(records, indent=2) + "\n")
    if code == 0:
        assert {r["severity"] for r in records} == {"critical", "high", "medium", "low"}
    return code


if __name__ == "__main__":
    raise SystemExit(main())
