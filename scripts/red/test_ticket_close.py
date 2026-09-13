"""Named CLI close regression and unchanged transition control."""

from scripts.red.test_board_cli import check


def test_close_uses_server_walk_and_preserves_refusal():
    check("./cmd", "TestTicketCloseUsesCanonicalClose")


def test_explicit_transition_remains_available_control():
    check("./internal/api", "TestTransitionTicket")
