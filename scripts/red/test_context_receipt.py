"""The actual Go command's receipt response, exposed as named pytest tests."""

from scripts.red.test_board_cli import check


def test_cli_replay_shows_the_receipt_deadline():
    check("./cmd", "TestContextLifecycle_ReplayReceiptDateIsVisible")


def test_cli_legacy_human_output_control():
    check("./cmd", "TestContextLifecycle_HumanOutput")


def test_cli_json_response_control():
    check("./cmd", "TestContextLifecycle_JSONIsTheAPIsObject")
