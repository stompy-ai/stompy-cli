"""Paste printed corrections into actual init/handwritten files, then recheck."""

import json
import re
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread
from urllib.parse import parse_qs, urlparse

import pytest

from scripts.red.test_output_formats_3738 import binary, invoke  # noqa: F401


@contextmanager
def rename_endpoint():
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            parsed = urlparse(self.path)
            requests.append(self.path)
            owner = parse_qs(parsed.query).get("owner", [""])[0]
            name = parsed.path.rsplit("/", 1)[-1]
            current = name.replace("old", "current", 1)
            body = {"name": current, "schema_name": "physical", "role": "owner"}
            if name.startswith("old"):
                prefix = f"@{owner}/" if owner else ""
                body.update(renamed_from=prefix + name, now_called=prefix + current)
            assert name in {"old", "current", "old2", "current2"}, self.path
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps(body).encode())

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


def correction_and_recheck(binary, directory, expected_line=None):
    marker = directory / ".stompy.yaml"
    original = marker.read_bytes().decode()
    with rename_endpoint() as (url, requests):
        before = invoke(binary, directory, url, "project", "check", "-o", "json")
        assert before.returncode == 0, before.stderr
        row = json.loads(before.stdout)[0]
        assert row["status"] == "renamed", row
        suggestion = row["marker_line"]
        table = invoke(binary, directory, url, "project", "check")
        assert table.returncode == 0 and suggestion in table.stdout, table
        assert marker.read_bytes().decode() == original
        lines = original.splitlines(keepends=True)
        indices = [i for i, line in enumerate(lines) if re.match(r"^\s*(?:-\s*)?address:", line)]
        assert len(indices) == 1
        line = lines[indices[0]].rstrip("\r\n")
        prefix, old_value = line.split("address:", 1)
        new_address = "@alice/current" if "/" in row["address"] else "current"
        quote = old_value.lstrip()[0] if old_value.lstrip()[0] in "\"'" else ""
        if not quote and "/" in row["address"]:
            new_address = "alice/current"
        want = prefix + "address: " + quote + new_address + quote
        if "# keep" in old_value:
            want += "  # keep"
        assert suggestion == (expected_line or want), (suggestion, want)
        ending = "\r\n" if lines[indices[0]].endswith("\r\n") else "\n"
        lines[indices[0]] = suggestion + ending
        marker.write_bytes("".join(lines).encode())
        after = invoke(binary, directory, url, "project", "check", "-o", "json")
        assert after.returncode == 0, (after.stdout, after.stderr, marker.read_text())
        assert json.loads(after.stdout)[0]["status"] == "ok"
        assert marker.read_bytes().decode() == "".join(lines)
        assert len(requests) == 3
        if "/" in row["address"]:
            assert all(parse_qs(urlparse(req).query) == {"owner": ["alice"]} for req in requests)
        else:
            assert all(not urlparse(req).query for req in requests)


@pytest.mark.parametrize("address", ["old", "@alice/old"])
def test_init_written_marker_correction_pastes_and_rechecks(binary, tmp_path, address):
    with rename_endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "init", "--project", address, "--role", "primary")
        assert result.returncode == 0 and requests == [], result
    original = (tmp_path / ".stompy.yaml").read_text()
    assert "    - address:" in original
    correction_and_recheck(binary, tmp_path)


@pytest.mark.parametrize("indent", [2, 4])
@pytest.mark.parametrize(
    "quote,address",
    [
        ("", "old"),
        ("", "alice/old"),
        ("'", "old"),
        ('"', "old"),
        ("'", "@alice/old"),
        ('"', "@alice/old"),
    ],
)
@pytest.mark.parametrize("address_first", [True, False])
def test_handwritten_correction_preserves_layout(
    binary, tmp_path, indent, quote, address, address_first
):
    spaces = " " * indent
    if address_first:
        body = f"{spaces}- address: {quote}{address}{quote}  # keep\n{spaces}  role: primary\n"
    else:
        body = f"{spaces}- role: primary\n{spaces}  address: {quote}{address}{quote}  # keep\n"
    (tmp_path / ".stompy.yaml").write_text(
        "version: 1\nprojects:\n" + body + spaces + "  default: true\n"
    )
    correction_and_recheck(binary, tmp_path)


@pytest.mark.parametrize(
    "entry",
    [
        "{address: old, role: primary, default: true}",
        "address: &bound old\n    role: primary\n    default: true",
    ],
)
def test_unbound_line_falls_back_to_value_instruction(binary, tmp_path, entry):
    marker = tmp_path / ".stompy.yaml"
    original = "version: 1\nprojects:\n  - " + entry + "\n"
    marker.write_text(original)
    with rename_endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
        assert result.returncode == 0, result
        row = json.loads(result.stdout)[0]
        assert row["status"] == "renamed" and row["now_called"] == "current"
        assert "marker_line" not in row
        table = invoke(binary, tmp_path, url, "project", "check")
        assert table.returncode == 0 and "replace the address value" in table.stdout, table
        assert '"old"' in table.stdout and '"current"' in table.stdout
        assert "address line with" not in table.stdout and marker.read_bytes().decode() == original


def test_canonical_control_has_no_replacement_and_never_writes(binary, tmp_path):
    marker = tmp_path / ".stompy.yaml"
    original = "version: 1\nprojects:\n  - address: current\n    role: primary\n    default: true\n"
    marker.write_text(original)
    with rename_endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
    assert result.returncode == 0 and json.loads(result.stdout)[0]["status"] == "ok"
    assert (
        "marker_line" not in json.loads(result.stdout)[0]
        and marker.read_bytes().decode() == original
    )
    assert requests == ["/projects/current"]


def test_invalid_marker_control_refuses_before_network_and_never_writes(binary, tmp_path):
    marker = tmp_path / ".stompy.yaml"
    original = "version: 1\nprojects: [broken\n"
    marker.write_text(original)
    with rename_endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", auth=False)
    assert result.returncode == 2 and requests == []
    assert marker.read_bytes().decode() == original and "marker_line" not in result.stdout


def test_CRLF_correction_pastes_without_changing_line_endings(binary, tmp_path):
    (tmp_path / ".stompy.yaml").write_bytes(
        b'version: 1\r\nprojects:\r\n    - address: "old"  # keep\r\n      role: primary\r\n      default: true\r\n'
    )
    correction_and_recheck(binary, tmp_path)


def test_each_correction_is_bound_to_its_own_entry(binary, tmp_path):
    marker = tmp_path / ".stompy.yaml"
    original = (
        "version: 1\nprojects:\n    - role: primary\n      address: 'old'\n      default: true\n"
        '    - address: "old2"  # second\n      role: reference\n'
    )
    marker.write_text(original)
    with rename_endpoint() as (url, requests):
        result = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
        assert result.returncode == 0, result
        rows = json.loads(result.stdout)
        assert [row.get("marker_line") for row in rows] == [
            "      address: 'current'",
            '    - address: "current2"  # second',
        ], rows
        assert marker.read_text() == original
        marker.write_text(
            original.replace("      address: 'old'", rows[0]["marker_line"]).replace(
                '    - address: "old2"  # second', rows[1]["marker_line"]
            )
        )
        after = invoke(binary, tmp_path, url, "project", "check", "-o", "json")
        assert after.returncode == 0 and [row["status"] for row in json.loads(after.stdout)] == [
            "ok",
            "ok",
        ], after
        assert requests == [
            "/projects/old",
            "/projects/old2",
            "/projects/current",
            "/projects/current2",
        ]
