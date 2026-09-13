"""Expose named Go behavioral checks through the build lane's pytest oracle."""

import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def check(package, name):
    result = subprocess.run(
        ["go", "test", package, "-run", "^" + name + "$", "-count=1"],
        cwd=ROOT, capture_output=True, text=True, timeout=30,
    )
    assert result.returncode == 0, result.stdout + result.stderr


def test_board_post_preserves_scope_identity_and_ttl():
    check("./cmd", "TestBoardPostCommandKeepsServerIdentityInJSON")


def test_board_read_quotes_untrusted_terminal_text():
    check("./cmd", "TestBoardReadCommandQuotesUntrustedTerminalText")


def test_board_api_post_preserves_plain_data_control():
    check("./internal/api", "TestPostBoardCarriesTextAndLabelButNotAuthorIdentity")


def test_board_api_read_preserves_filters_control():
    check("./internal/api", "TestReadBoardCarriesFiltersAndServerWeights")
