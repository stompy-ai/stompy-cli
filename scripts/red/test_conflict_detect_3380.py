"""Conflict scan truth through the compiled CLI and synthetic REST responses."""

import json
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread
from urllib.parse import parse_qs, urlsplit

import pytest
from test_output_formats_3738 import binary, invoke  # noqa: F401


def scan(status="completed", found=2):
    return {
        "status": status,
        "conflicts_found": found,
        "auto_resolved": 0,
        "pending": found,
        "processing_time_ms": 20048,
        "pairs_examined": 407,
        "pairs_total": 435,
        "pairs_unexamined": 28,
    }


def incomplete(reason="time_budget", found=2):
    return {
        **scan("incomplete", found),
        "reason": reason,
        "hint": "Rerun a narrower scan; absence is not proof of no conflicts.",
        "unjudged_pairs": 1,
        "unjudged_ids": [[31, 32]],
        "likely_unconfirmed_pairs": 2,
        "likely_unconfirmed_ids": [[25, 26], [27, 28]],
        "llm_escalations_truncated": True,
    }


@contextmanager
def detection_endpoint(payload, status=200):
    calls = []

    class DetectionHandler(BaseHTTPRequestHandler):
        def do_POST(self):
            calls.append(
                (
                    self.path,
                    self.headers.get("Authorization"),
                    json.loads(self.rfile.read(int(self.headers["Content-Length"]))),
                )
            )
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(
                payload if isinstance(payload, bytes) else json.dumps(payload).encode()
            )

        def log_message(self, *args):
            pass

    with ThreadingHTTPServer(("127.0.0.1", 0), DetectionHandler) as server:
        task = Thread(
            target=lambda: server.serve_forever(poll_interval=0.01), daemon=True
        )
        task.start()
        try:
            yield f"http://127.0.0.1:{server.server_port}", calls
        finally:
            server.shutdown()
            task.join(timeout=5)
            assert not task.is_alive()


def detect(
    binary, tmp_path, payload, *flags, scope="recent", status=200, extra_env=None
):
    with detection_endpoint(payload, status) as (url, calls):
        result = invoke(
            binary,
            tmp_path,
            url,
            "conflict",
            "detect",
            "-p",
            "@alice/demo",
            "--scope",
            scope,
            *flags,
            extra_env=extra_env,
        )
    assert len(calls) == 1, calls
    path, authorization, request = calls[0]
    assert urlsplit(path).path == "/conflicts/detect"
    assert parse_qs(urlsplit(path).query) == {"project_id": ["@alice/demo"]}
    assert authorization == "Bearer synthetic" and request == {"scope": scope}
    return result


@pytest.mark.parametrize(
    "reason", ["time_budget", "llm_escalation_cap", "provider_unavailable"]
)
@pytest.mark.parametrize("found", [0, 2])
def test_incomplete_table_names_reason_progress_hint_and_candidates(
    binary, tmp_path, reason, found
):
    payload = incomplete(reason, found)
    result = detect(binary, tmp_path, payload)
    assert result.returncode == 1, (result.stdout, result.stderr)
    assert "✓" not in result.stdout + result.stderr
    assert "incomplete" in result.stdout.lower() and reason in result.stdout
    assert "407 of 435" in result.stdout and f"Found {found} conflicts" in result.stdout
    assert payload["hint"] in result.stdout
    assert (
        "31,32" in result.stdout
        and "25,26" in result.stdout
        and "27,28" in result.stdout
    )
    assert "Unjudged" in result.stdout and "Likely unconfirmed" in result.stdout
    assert result.stderr == ""


@pytest.mark.parametrize("kind", ["incomplete", "completed", "legacy", "queued"])
@pytest.mark.parametrize("format_source", ["flag", "normalized", "config"])
def test_json_preserves_whole_server_object(binary, tmp_path, kind, format_source):
    payload = incomplete() if kind == "incomplete" else scan()
    if kind == "legacy":
        payload.pop("status")
    if kind == "queued":
        payload = {
            "status": "queued",
            "job_id": "job-17",
            "scope": "all",
            "message": "Scan queued",
        }
    payload.update(
        {"future": {"ids": [9007199254740993], "text": "apiary 🐝"}, "nullable": None}
    )
    flags = ("-o", " json ") if format_source == "normalized" else ("-o", "json")
    env = None
    if format_source == "config":
        flags, env = (), {"STOMPY_OUTPUT_FORMAT": "JSON"}
    result = detect(
        binary,
        tmp_path,
        payload,
        *flags,
        extra_env=env,
        scope="all" if kind == "queued" else "recent",
        status=202 if kind == "queued" else 200,
    )
    assert json.loads(result.stdout) == payload, result.stdout
    assert result.returncode == (1 if kind == "incomplete" else 0), result.stderr
    assert result.stderr == "" and "✓" not in result.stdout


@pytest.mark.parametrize("status", ["completed", "legacy"])
def test_completed_table_control_retains_success(binary, tmp_path, status):
    payload = scan()
    if status == "legacy":
        payload.pop("status")
    result = detect(binary, tmp_path, payload)
    assert (
        result.returncode == 0
        and "✓" in result.stdout
        and "Found 2 conflicts" in result.stdout
    )
    assert "0 auto-resolved, 2 pending" in result.stdout and "20048ms" in result.stdout
    assert "incomplete" not in result.stdout.lower() and result.stderr == ""


def test_queued_table_control_does_not_claim_completed_scan(binary, tmp_path):
    result = detect(
        binary,
        tmp_path,
        {"status": "queued", "job_id": "job-17", "message": "Scan queued"},
        scope="all",
        status=202,
    )
    assert result.returncode == 0 and "Scan queued (job job-17)" in result.stdout
    assert (
        "Found" not in result.stdout
        and "✓" not in result.stdout
        and result.stderr == ""
    )


@pytest.mark.parametrize("status", ["completed", "incomplete"])
def test_yaml_honors_scan_status_and_preserves_extra_fields(binary, tmp_path, status):
    payload = incomplete() if status == "incomplete" else scan()
    payload["future_field"] = "kept"
    result = detect(binary, tmp_path, payload, "-o", "yaml")
    assert (
        f"status: {status}" in result.stdout and "future_field: kept" in result.stdout
    )
    assert (
        "pairs_examined: 407" in result.stdout and "pairs_total: 435" in result.stdout
    )
    assert result.returncode == (1 if status == "incomplete" else 0)
    assert result.stderr == "" and "✓" not in result.stdout


@pytest.mark.parametrize("missing", ["pairs_examined", "pairs_total", "both"])
def test_missing_progress_is_unknown_not_invented_zero(binary, tmp_path, missing):
    payload = incomplete("provider_unavailable", 0)
    for key in ("pairs_examined", "pairs_total"):
        if missing in (key, "both"):
            payload.pop(key)
    result = detect(binary, tmp_path, payload)
    expected = {
        "pairs_examined": "unknown of 435",
        "pairs_total": "407 of unknown",
        "both": "unknown of unknown",
    }
    assert expected[missing] in result.stdout and result.returncode == 1
    assert "✓" not in result.stdout and result.stderr == ""


def test_zero_progress_is_not_unknown(binary, tmp_path):
    payload = incomplete()
    payload.update(pairs_examined=0, pairs_total=0)
    result = detect(binary, tmp_path, payload)
    assert result.returncode == 1 and "0 of 0" in result.stdout
    assert "unknown" not in result.stdout and "✓" not in result.stdout


def test_sparse_provider_outage_never_claims_success(binary, tmp_path):
    payload = {
        "status": "incomplete",
        "reason": "provider_unavailable",
        "unjudged_pairs": 1,
        "unjudged_ids": [[31, 32]],
        "conflicts_found": 0,
    }
    result = detect(binary, tmp_path, payload)
    assert result.returncode == 1 and "provider_unavailable" in result.stdout
    assert "31,32" in result.stdout and "unknown of unknown" in result.stdout
    assert "✓" not in result.stdout + result.stderr


@pytest.mark.parametrize(
    "payload,status,expected",
    [
        ({"detail": "NLI service not available"}, 503, "NLI service not available"),
        (b"not-json", 200, "decoding response"),
    ],
)
def test_transport_and_decode_failure_controls(
    binary, tmp_path, payload, status, expected
):
    result = detect(binary, tmp_path, payload, "-o", "json", status=status)
    assert result.returncode == 1 and expected in result.stderr and result.stdout == ""
