import pytest

from app.formatting import (
    explorer_tx_link,
    normalize_address,
    unix_to_iso_utc,
    wei_to_eth_string,
)


@pytest.mark.parametrize(
    "wei, expected",
    [
        (0, "0.000000000000000000"),
        (1, "0.000000000000000001"),
        (1_500_000_000_000_000_000, "1.500000000000000000"),
        (10**18, "1.000000000000000000"),
        # bigger than float precision can hold: must stay exact
        (123_456_789_012_345_678_901, "123.456789012345678901"),
    ],
)
def test_wei_to_eth_string_is_exact(wei, expected):
    assert wei_to_eth_string(wei) == expected


def test_wei_negative_rejected():
    with pytest.raises(ValueError):
        wei_to_eth_string(-1)


def test_unix_to_iso_utc():
    assert unix_to_iso_utc(0) == "1970-01-01T00:00:00Z"
    assert unix_to_iso_utc(1_789_583_400) == "2026-09-16T18:30:00Z"


def test_normalize_address_lowercases_and_trims():
    assert (
        normalize_address("  0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed ")
        == "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"
    )


def test_explorer_link_handles_missing_trailing_slash():
    assert explorer_tx_link("https://etherscan.io/tx", "0xabc") == "https://etherscan.io/tx/0xabc"
    assert explorer_tx_link("https://etherscan.io/tx/", "0xabc") == "https://etherscan.io/tx/0xabc"
