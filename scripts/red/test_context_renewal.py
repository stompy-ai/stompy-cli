"""Expose the actual Go command behavior through a named pytest oracle."""

from scripts.red.test_board_cli import check


def test_cli_keeps_the_callers_renewal_key_on_retries():
    check("./cmd", "TestContextLifecycle_RenewalKeySurvivesRetries")


def test_cli_without_renewal_keeps_the_legacy_request_control():
    check("./cmd", "TestContextLifecycle_WithoutRenewalKeyKeepsLegacyRequest")


def test_cli_rejects_invalid_renewal_before_rest():
    check("./cmd", "TestContextLifecycle_RenewalKeyRequiresParkedAndExplicitDays")
