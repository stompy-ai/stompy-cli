"""Compiled marker diagnostics, with synthetic auth and loopback readbacks only."""

import json
import os
import subprocess
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Thread
from urllib.parse import parse_qs, urlparse

import pytest

ROOT = Path(__file__).resolve().parents[2]
HINT = 'quote addresses that start with @, e.g. address: "@handle/name"'
ALLOWED = "(allowed keys: version, environment, projects[address, role, default, purpose])"
MARKER = "version: 1\nprojects:\n  - address: @handle/name\n    role: primary\n    default: true\n"


@pytest.fixture(scope="session")
def binary(tmp_path_factory):
    target = tmp_path_factory.mktemp("address-hint-cli") / "stompy"
    subprocess.run(["go", "build", "-o", str(target), "./cmd/stompy"], cwd=ROOT, check=True)
    return target


@contextmanager
def endpoint():
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            requests.append(self.path)
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"name": "name", "role": "owner"}).encode())

        def log_message(self, *args):
            pass

    with ThreadingHTTPServer(("127.0.0.1", 0), Handler) as server:
        worker = Thread(target=lambda: server.serve_forever(poll_interval=0.01), daemon=True)
        worker.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}", requests
        finally:
            server.shutdown()
            worker.join(timeout=5)
            assert not worker.is_alive()


def invoke(binary, directory, url, *args):
    # With HOME absent, config falls back to this temporary cwd's .stompy.
    return subprocess.run(
        [str(binary), "--api-url", url, "--api-key", "synthetic", *args],
        cwd=directory,
        env={"PATH": os.environ["PATH"]},
        capture_output=True,
        text=True,
        timeout=15,
    )


def check_invalid(binary, directory, source, args=("project", "check"), code=2):
    marker = directory / ".stompy.yaml"
    original = source.encode()
    marker.write_bytes(original)
    assert marker.read_bytes() == original
    with endpoint() as (url, requests):
        result = invoke(binary, directory, url, *args)
        assert result.returncode == code, result
        assert requests == [] and result.stdout == "", result
    assert marker.read_bytes() == original
    assert not (directory / "AGENTS.md").exists()
    assert not (directory / "CLAUDE.md").exists()
    return result.stderr


@pytest.mark.parametrize(
    "line",
    ["  - address: @handle/name", "    address: @handle/name", '  - "address": @handle/name', "  - 'address':\t@handle/name"],
    ids=["list", "after-role", "quoted-key", "tab-value"],
)
@pytest.mark.parametrize("ending", ["\n", "\r\n"], ids=["lf", "crlf"])
def test_unquoted_address_hint_keeps_cause_and_exit_two(binary, tmp_path, line, ending):
    prefix = "version: 1\nprojects:\n"
    if line.startswith("    address"):
        prefix += "  - role: primary\n"
    source = (prefix + line + "\n    default: true\n").replace("\n", ending)
    error = check_invalid(binary, tmp_path, source)
    number = len(prefix.splitlines()) + 1
    assert f"yaml: line {number}: found character that cannot start any token" in error
    assert ALLOWED in error
    assert error.count(HINT) == 1, error


@pytest.mark.parametrize("command", [("context", "list"), ("init",)], ids=["context", "init"])
def test_shared_marker_loaders_keep_their_exit_and_hint(binary, tmp_path, command):
    error = check_invalid(binary, tmp_path, MARKER, command, code=1)
    assert "yaml: line 3: found character that cannot start any token" in error
    assert ALLOWED in error and error.count(HINT) == 1, error


def test_hint_quoted_example_pastes_and_rechecks(binary, tmp_path):
    error = check_invalid(binary, tmp_path, MARKER)
    assert HINT in error
    example = error.split("e.g. ", 1)[1].splitlines()[0]
    assert example == 'address: "@handle/name"'
    quoted = MARKER.replace("address: @handle/name", example)
    marker = tmp_path / ".stompy.yaml"
    marker.write_text(quoted)
    assert marker.read_text() == quoted
    with endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
        assert result.returncode == 0 and HINT not in result.stderr, result
        assert json.loads(result.stdout)[0]["status"] == "ok"
        assert len(requests) == 1
        parsed = urlparse(requests[0])
        assert parsed.path.endswith("/projects/name")
        assert parse_qs(parsed.query) == {"owner": ["handle"]}
    assert marker.read_text() == quoted


@pytest.mark.parametrize("quote", ["'", '"'], ids=["single", "double"])
def test_quoted_address_control_resolves_without_hint(binary, tmp_path, quote):
    source = MARKER.replace("@handle/name", quote + "@handle/name" + quote)
    marker = tmp_path / ".stompy.yaml"
    marker.write_text(source)
    with endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
        assert result.returncode == 0 and HINT not in result.stderr, result
        assert json.loads(result.stdout)[0]["status"] == "ok"
        assert requests == ["/projects/name?owner=handle"]
    assert marker.read_text() == source


@pytest.mark.parametrize(
    "source",
    [
        "version: [\n",
        MARKER.replace("address: @handle/name", 'address: name\n    purpose: @unquoted'),
        "version: 1\nenvironment: @wrong\n" + MARKER.split("\n", 1)[1],
        'version: @wrong # address: @handle/name\n',
        'version: "address: @handle/name\n',
        MARKER.replace("@handle/name", '"@handle/name"').replace("version: 1", "version: 2"),
        "",
    ],
    ids=["unrelated", "non-address", "earlier-line", "comment", "quoted-text", "validation", "empty"],
)
def test_other_errors_control_gets_no_hint(binary, tmp_path, source):
    error = check_invalid(binary, tmp_path, source)
    assert HINT not in error
    assert ".stompy.yaml" in error


def test_unrelated_parse_cause_stays_intact(binary, tmp_path):
    error = check_invalid(binary, tmp_path, "version: [\n")
    assert "yaml: line 1: did not find expected node content" in error
    assert ALLOWED in error
