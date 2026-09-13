"""Named CLI lease regressions and the existing canonical-close control."""

from scripts.red.test_board_cli import check


def test_lease_actions_keep_owner_label_and_single_write():
    check("./cmd", "TestTicketLeaseCommandsUseOneCanonicalWrite")


def test_lease_refusal_is_not_success():
    check("./cmd", "TestTicketLeaseRefusalNeverPrintsSuccess")


def test_canonical_close_remains_available_control():
    check("./cmd", "TestTicketCloseUsesCanonicalClose")
