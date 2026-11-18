"""The real binary validates formats before commands, auth or HTTP requests."""

import json
import os
import subprocess
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread

import pytest

ROOT = Path(__file__).resolve().parents[2]
MARKER = 'version: 1\nenvironment: production\nprojects:\n  - address: "@alice/old"\n    role: primary\n    default: true\n'


@pytest.fixture(scope="session")
def binary(tmp_path_factory):
    target = tmp_path_factory.mktemp("cli") / "stompy"
    subprocess.run(["go", "build", "-o", str(target), "./cmd/stompy"], cwd=ROOT, check=True)
    return target


@contextmanager
def endpoint():
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append(self.path)
            missing = "missing" in self.path
            self.send_response(404 if missing else 200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            body = (
                {"detail": "missing"}
                if missing
                else {
                    "name": "current",
                    "schema_name": "physical",
                    "role": "owner",
                    "home": "personal",
                    "address": "alice/current",
                    "renamed_from": "@alice/old",
                    "now_called": "@alice/current",
                }
            )
            if "/projects/current?" in self.path:
                body.pop("renamed_from")
                body.pop("now_called")
            self.wfile.write(json.dumps(body).encode())

        def log_message(self, *args):
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}", requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)
        assert not thread.is_alive()


def invoke(binary, directory, url, *args, extra_env=None, auth=True):
    home = directory / "isolated-home"
    home.mkdir(exist_ok=True)
    env = {"PATH": os.environ["PATH"], "HOME": str(home), "STOMPY_API_KEY": ""}
    env.update(extra_env or {})
    return subprocess.run(
        [str(binary), "--api-url", url, *(["--api-key", "synthetic"] if auth else []), *args],
        cwd=directory,
        env=env,
        capture_output=True,
        text=True,
        timeout=15,
    )


@pytest.mark.parametrize("format", ["JSON", "Json", " json "])
@pytest.mark.parametrize("suffix", [False, True])
def test_compiled_check_normalizes_json_and_preserves_qualified_binding(
    binary, tmp_path, format, suffix
):
    marker = tmp_path / ".stompy.yaml"
    marker.write_text(MARKER)
    args = ["project", "check", "-o", format] if suffix else ["-o", format, "project", "check"]
    with endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, *args)
    assert result.returncode == 0, result.stderr
    rows = json.loads(result.stdout)
    assert rows[0]["status"] == "renamed" and rows[0]["now_called"] == "@alice/current"
    assert rows[0]["marker_line"] == '  - address: "@alice/current"'
    assert rows[0]["address"] == "@alice/old" and rows[0]["caller_role"] == "owner"
    assert requests == ["/projects/old?owner=alice"]
    assert 'Project is now called "@alice/current".' in result.stderr
    assert marker.read_text() == MARKER


@pytest.mark.parametrize("command", [["project", "check"], ["context", "list"], ["version"]])
def test_unknown_format_is_exit_two_before_auth_or_network(binary, tmp_path, command):
    (tmp_path / ".stompy.yaml").write_text(MARKER)
    with endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, *command, "-o", "wide", auth=False)
    assert result.returncode == 2, (result.stdout, result.stderr)
    assert 'unknown output format "wide": use table, json or yaml' in result.stderr
    assert result.stdout == "" and requests == []


def test_configured_format_is_normalized_and_validated(binary, tmp_path):
    (tmp_path / ".stompy.yaml").write_text(MARKER)
    with endpoint() as (url, requests):
        result = invoke(
            binary, tmp_path, url, "project", "check", extra_env={"STOMPY_OUTPUT_FORMAT": "JSON"}
        )
        assert result.returncode == 0 and json.loads(result.stdout)[0]["status"] == "renamed"
        result = invoke(
            binary, tmp_path, url, "version", extra_env={"STOMPY_OUTPUT_FORMAT": "wide"}
        )
    assert result.returncode == 2 and 'unknown output format "wide"' in result.stderr
    assert requests == ["/projects/old?owner=alice"]


def test_lowercase_json_mixed_failure_control(binary, tmp_path):
    marker = MARKER + "  - address: missing\n    role: reference\n"
    (tmp_path / ".stompy.yaml").write_text(marker)
    with endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
    assert result.returncode == 1
    assert [r["status"] for r in json.loads(result.stdout)] == ["renamed", "not_found"]
    assert len(requests) == 2 and (tmp_path / ".stompy.yaml").read_text() == marker


def test_table_control_preserves_actionable_correction(binary, tmp_path):
    (tmp_path / ".stompy.yaml").write_text(MARKER)
    with endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check")
    assert result.returncode == 0 and "RENAMED" in result.stdout
    assert '  - address: "@alice/current"' in result.stdout and len(requests) == 1


def test_canonical_json_control_has_no_rename_or_marker_edit(binary, tmp_path):
    marker = MARKER.replace("@alice/old", "@alice/current")
    (tmp_path / ".stompy.yaml").write_text(marker)
    with endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
    assert result.returncode == 0
    row = json.loads(result.stdout)[0]
    assert row["status"] == "ok" and "now_called" not in row and "marker_line" not in row
    assert "Project is now called" not in result.stderr
    assert len(requests) == 1 and (tmp_path / ".stompy.yaml").read_text() == marker
