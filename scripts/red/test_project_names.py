"""Named behavioral proof through the actual Go command and REST client."""

import json
import subprocess
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]


@pytest.fixture(scope="session")
def results():
    run = subprocess.run(
        ["go", "test", "./cmd", "./internal/api", "-json", "-count=1",
         "-timeout=120s", "-run", "^Test3709|^TestClient_Do_RetriesOn502$"],
        cwd=ROOT, capture_output=True, text=True, timeout=180,
    )
    assert run.returncode in (0, 1), run.stdout + run.stderr
    events = [json.loads(line) for line in run.stdout.splitlines()]
    statuses = {e["Test"]: e["Action"] for e in events
                if "Test" in e and e["Action"] in {"pass", "fail", "skip"}}
    assert statuses and not any(e.get("Action") == "build-fail" for e in events), run.stdout + run.stderr
    packages = {e["Package"] for e in events
                if "Test" not in e and e.get("Action") in {"pass", "fail"}}
    assert packages == {"github.com/stompy-ai/stompy-cli/cmd",
                        "github.com/stompy-ai/stompy-cli/internal/api"}, run.stdout + run.stderr
    failed_packages = {e["Package"] for e in events
                       if "Test" not in e and e.get("Action") == "fail"}
    failed_tests = {e["Package"] for e in events
                    if "Test" in e and e.get("Action") == "fail"}
    assert failed_packages == failed_tests, "package failure outside a test: " + run.stdout + run.stderr
    assert "skip" not in statuses.values(), run.stdout + run.stderr
    return statuses, run.stdout + run.stderr


def proved(results, *names):
    statuses, output = results
    assert all(name in statuses for name in names), "missing test collection: " + output
    assert all(statuses[name] == "pass" for name in names), output


def test_notice_on_every_rest_command(results):
    proved(results, "Test3709AliasNoticeAllRESTCommands")


def test_notice_quotes_untrusted_text(results):
    proved(results, "Test3709AliasNoticeQuotesTerminalText")


def test_authored_data_and_refusal_control(results):
    proved(results, "Test3709AliasNoticeIgnoresRefusalsAndAuthoredData")


def test_marker_correction_and_qualified_scope(results):
    proved(results, "Test3709CheckRenamedPreservesMarkerAndScope", "Test3709CheckRenamedJSON")


def test_personal_marker_correction(results):
    proved(results, "Test3709CheckPersonalCorrection")


def test_mixed_marker_failure_stays_failure(results):
    proved(results, "Test3709CheckRenamedDoesNotHideMissing")


def test_unverifiable_marker_evidence_refused(results):
    proved(results, "Test3709CheckUnverifiableNoticeDoesNotSuggestAddress")


def test_canonical_marker_control(results):
    proved(results, "Test3709CheckCanonicalControl")


def test_qualified_rename_round_trip(results):
    proved(results, "Test3709RenameQualifiedRoundTrip")


def test_recreated_name_cannot_verify(results):
    proved(results, "Test3709RenameRecreateRefusesVerifiedSuccess")


def test_recreated_schema_cannot_verify(results):
    proved(results, "Test3709RenameSameSchemaRecreateCannotVerify")


def test_later_name_cannot_verify(results):
    proved(results, "Test3709RenameLaterNameDoesNotBecomeSuccess")


def test_live_alias_readback_required(results):
    proved(results, "Test3709RenameAliasReadbackFailureIsVisible")


def test_committed_pending_control(results):
    proved(results, "Test3709RenamePendingIsCommittedSuccess")


def test_owner_refusal_control(results):
    proved(results, "Test3709RenameRefusalControl")


def test_arity_control(results):
    proved(results, "Test3709RenameArityNoNetwork", "Test3709RenameEmptyInputNoNetwork")


def test_name_endpoint_authentication_control(results):
    proved(results, "Test3709NameEndpointsOwnerAndAuthentication", "Test3709RenameRequestPreservesNewName")


def test_release_must_not_retry(results):
    proved(results, "Test3709ReleaseTransportNeverRetries")


def test_existing_read_retry_control(results):
    proved(results, "TestClient_Do_RetriesOn502")


def test_release_round_trip_and_binding(results):
    proved(results, "Test3709ReleaseQualifiedRoundTrip", "Test3709ReleaseUsesMarkerBindingWithoutRewriting")


def test_release_requires_confirmation(results):
    proved(results, "Test3709ReleaseNeedsConfirmationBeforeNetwork")


def test_release_retained_alias_refused(results):
    proved(results, "Test3709ReleaseRetainedAliasCannotVerify")


def test_release_changed_readback_refused(results):
    proved(results, "Test3709ReleaseRejectsChangedReadback")


def test_release_refusal_control(results):
    proved(results, "Test3709ReleaseRefusalControl", "Test3709ReleaseEmptyProjectNoNetwork")


def test_aliases_current_sorted_and_empty(results):
    proved(results, "Test3709AliasesQualifiedSortedAndCurrentName", "Test3709AliasesEmptyIsExplicit")


def test_aliases_unverifiable_list_refused(results):
    proved(results, "Test3709AliasesMissingEvidenceNotEmpty")


def test_aliases_owner_refusal_control(results):
    proved(results, "Test3709AliasesRefusalNotEmpty", "Test3709AliasesArityNoNetwork")
