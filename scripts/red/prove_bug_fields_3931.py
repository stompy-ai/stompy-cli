"""From a backend worktree, use local_ci run -- python THIS --binary CANDIDATE.

--expect-old-artifact checks exact native RED with the backend's e091339c CLI.
All writes are the backend's disposable MAIN/routed fixtures; auth alone is synthetic.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import os
import subprocess
import sys
from pathlib import Path
from typing import Any, Generator
from xml.etree import ElementTree as ET

import pytest
import yaml

RECORDS = []
PRODUCER = "test_submit_has_one_main_target_with_or_without_project"
MODULE = "tests/integration/test_3834_cutover_doors.py"


async def readback(case: Any, home: Path, binary: Path, nodeid: str) -> None:
    from fastapi.testclient import TestClient
    from src.contextvars import current_user_tier
    from tests.conftest import parse_tool_response
    from tests.integration.fixtures_2210 import one, run
    from tests.integration.fixtures_3705_doors import running
    from tests.integration.fixtures_3834_cutover import counts, seed

    number = one(case.w.main, "SELECT id FROM bug_inbox.tickets")
    report_id = f"BUG-{number}"
    repeated = parse_tool_response(await case.host.bug_report(
        title="cutover target", description="one durable row", severity="critical",
        steps_to_reproduce="click", expected_behavior="works", actual_behavior="stops"))
    metadata = json.loads(one(case.w.main,
        "SELECT metadata FROM bug_inbox.tickets WHERE id=%s", (number,)))
    assert repeated["id"] == report_id and repeated["submission_status"] == "duplicate"
    assert metadata["occurrences"] == 2 and metadata["reporter_user_id"] == case.w.grantee_id
    assert counts(case) == (1, 0, 0, 0)
    foreign = seed(case, reporter=case.w.owner_id, title="private reporter sentinel")
    with TestClient(case.app) as client:
        ack = client.post("/api/v1/errors/report", json={
            "reference_id": "ERR-3931", "message": "3931 anonymous native telemetry"})
        assert ack.status_code == 202 and ack.json()["status"] == "created"
        assert "id" not in ack.json()
        anonymous = one(case.w.main,
            "SELECT id FROM bug_inbox.tickets WHERE title LIKE %s", ("[Client Error ERR-3931]%",))
        own_rest = client.get("/api/v1/bug-reports/" + report_id)
        assert own_rest.status_code == 200 and own_rest.json()["occurrences"] == 2
        assert client.get(f"/api/v1/bug-reports/BUG-{anonymous}").status_code == 404
    own = parse_tool_response(await case.host.bug_report_get(report_id=report_id))
    assert own_rest.json() == own and own["reporter_user_id"] == case.w.grantee_id
    before = run(case.w.main, "SELECT * FROM bug_inbox.tickets ORDER BY id", fetch=True)
    assert counts(case) == (3, 0, 0, 0)

    with running(case.app) as url:
        command = [str(binary), "--api-url", url, "--api-key", "synthetic"]
        env = {"PATH": "/usr/bin:/bin", "HOME": str(home), "NO_COLOR": "1"}

        def call(*args: str, output: str = "json") -> subprocess.CompletedProcess[str]:
            return subprocess.run(command + ["-o", output, "bug", *args],
                                  cwd=home, env=env, capture_output=True, text=True, timeout=20)

        def decoded(result: subprocess.CompletedProcess[str], output: str) -> Any:
            assert result.returncode == 0, result.stdout + result.stderr
            return json.loads(result.stdout) if output == "json" else yaml.safe_load(result.stdout)

        def observation(row: dict[str, Any], reporter: int | str, occurrences: int) -> None:
            for output in ("json", "yaml", "table"):
                result = call("get", row["id"], output=output)
                assert result.returncode == 0, result.stdout + result.stderr
                if output == "table":
                    for key, value in (("Reporter", reporter), ("Occurrences", occurrences)):
                        assert any(key + ":" in line and str(value) in line
                                   for line in result.stdout.splitlines()), result.stdout
                else:
                    got = decoded(result, output)
                    assert got["ID"] == row["id"]
                    assert got["Reporter"] == reporter and got["Occurrences"] == occurrences
                    assert type(got["Reporter"]) is type(reporter)
                    assert type(got["Occurrences"]) is int
                    assert got["Environment"] == row["environment"]
                    assert got["Project Name"] == (row["project_name"] or "")
                    assert got["Updated"] == row["updated_at"]

        try:
            observation(own, case.w.grantee_id, 2)
            for output in ("json", "yaml", "table"):
                result = call("list", "--severity", "critical", output=output)
                assert result.returncode == 0, result.stdout + result.stderr
                if output == "table":
                    assert "PROJECT" in result.stdout and "UPDATED" in result.stdout
                    assert report_id in result.stdout
                else:
                    rows = decoded(result, output)
                    assert len(rows) == 1 and rows[0]["ID"] == report_id
                    assert rows[0]["PROJECT"] == (own["project_name"] or "")
                    assert rows[0]["UPDATED"] == own["updated_at"]
            denied = call("get", foreign["id"])
            absent = call("get", "BUG-2147483647")
            assert denied.returncode == absent.returncode != 0
            assert denied.stdout == absent.stdout == "" and denied.stderr == absent.stderr
            assert "404" in denied.stderr and foreign["title"] not in denied.stderr
            denied_mcp = parse_tool_response(await case.host.bug_report_get(report_id=foreign["id"]))
            assert denied_mcp["error"] == "NOT_FOUND"
            with TestClient(case.app) as client:
                assert client.get("/api/v1/bug-reports/" + foreign["id"]).status_code == 404
            case.principal.tier = "admin"
            token = current_user_tier.set("admin")
            try:
                anon = parse_tool_response(await case.host.bug_report_get(report_id=f"BUG-{anonymous}"))
                with TestClient(case.app) as client:
                    rest = client.get(f"/api/v1/bug-reports/BUG-{anonymous}")
                    assert rest.status_code == 200 and rest.json() == anon
                assert anon["reporter_user_id"] == "anonymous" and anon["occurrences"] == 1
                observation(anon, "anonymous", 1)
                staff = decoded(call("get", foreign["id"]), "json")
                assert staff["Reporter"] == case.w.owner_id and staff["Occurrences"] == 1
            finally:
                current_user_tier.reset(token)
                case.principal.tier = "beta"
            RECORDS.append({"case": nodeid, "id": report_id,
                            "reporter": case.w.grantee_id, "occurrences": 2,
                            "project": own["project_name"], "updated": own["updated_at"],
                            "anonymous": anon["id"], "native_all_three_doors": True,
                            "cli_formats": ["json", "yaml", "table"],
                            "foreign_equals_absent": True, "physical_counts": [3, 0, 0, 0]})
        finally:
            assert run(case.w.main, "SELECT * FROM bug_inbox.tickets ORDER BY id", fetch=True) == before
            assert counts(case) == (3, 0, 0, 0)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--expect-old-artifact", action="store_true")
    args = parser.parse_args()
    assert os.environ.get("TEST_DATABASE_URL"), "run with backend local_ci"
    assert Path(MODULE).exists(), "run from a backend worktree"
    binary = args.binary.resolve()
    build = subprocess.check_output(["go", "version", "-m", str(binary)], text=True)
    assert "vcs.modified=false" in build, "use a clean committed CLI artifact"
    if args.expect_old_artifact:
        assert "vcs.revision=e091339c40c610b2d06b325cce49cfde001258db" in build
    args.out.mkdir(parents=True, exist_ok=True)
    (args.out / "cli-build.txt").write_text(build)
    sys.path.insert(0, str(Path.cwd()))
    os.environ["STOMPY_CLI_BIN"] = str(binary)

    class Probe:
        def pytest_collection_modifyitems(self, items: list[Any]) -> None:
            for item in items:
                if "tmp_path" not in item.fixturenames:
                    item.fixturenames.append("tmp_path")

        @pytest.hookimpl(wrapper=True)
        def pytest_runtest_call(self, item: Any) -> Generator[None, Any, Any]:
            result = yield
            if item.originalname == PRODUCER:
                asyncio.run(readback(item.funcargs["doors"], item.funcargs["tmp_path"], binary, item.nodeid))
            return result

    xml = args.out / "native.xml"
    code = pytest.main([MODULE + "::" + PRODUCER,
                        MODULE + "::test_other_and_absent_refuse_identically_both_doors",
                        MODULE + "::test_pinned_cli_reads_own_and_refuses_other_without_new_store",
                        "-v", "--junitxml=" + str(xml)], plugins=[Probe()])
    rows = list(ET.parse(xml).iter("testcase"))
    failed = {r.attrib["classname"] + "::" + r.attrib["name"] for r in rows if r.find("failure") is not None}
    assert not [r for r in rows if r.find("error") is not None or r.find("skipped") is not None]
    prefix = "tests.integration.test_3834_cutover_doors::"
    expected = {prefix + PRODUCER + "[" + p + "]" for p in ("None", "foo", "old-alias", "missing-project")}
    controls = {prefix + name for name in (
        "test_other_and_absent_refuse_identically_both_doors",
        "test_pinned_cli_reads_own_and_refuses_other_without_new_store")}
    passed = {r.attrib["classname"] + "::" + r.attrib["name"] for r in rows if r.find("failure") is None}
    assert controls <= passed
    assert code == (1 if args.expect_old_artifact else 0)
    assert failed == (expected if args.expect_old_artifact else set()), failed
    if not args.expect_old_artifact:
        assert len(RECORDS) == 4
    proof = {"pytest_exit": int(code), "failed": sorted(failed), "green_controls": sorted(controls), "records": RECORDS}
    (args.out / "readback.json").write_text(json.dumps(proof, indent=2))
    print(json.dumps(proof, indent=2))


if __name__ == "__main__":
    main()
