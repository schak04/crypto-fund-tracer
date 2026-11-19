"""Small, pure helper functions that turn raw chain data into contract formats.

"Pure" = same input always gives the same output, no network, no state. That
makes them trivial to unit-test, and they are where precision bugs would hide.
"""

from __future__ import annotations

import re
from datetime import datetime, timezone

from app.config import WEI_PER_ETH

_ADDRESS_RE = re.compile(r"^0x[0-9a-f]{40}$")
_ETH_DECIMALS = 18


def normalize_address(address: str) -> str:
    """Canonical internal form: stripped, lowercase, '0x' prefix.

    Ethereum addresses are case-insensitive hex; mixed case only encodes an
    EIP-55 checksum. Using one canonical form everywhere means 0xABC... and
    0xabc... can never be treated as two different wallets.
    """
    cleaned = address.strip().lower()
    if cleaned.startswith("0x"):
        pass
    elif len(cleaned) == 40:
        cleaned = "0x" + cleaned
    return cleaned


def is_valid_address(address: str) -> bool:
    return bool(_ADDRESS_RE.match(address))


def wei_to_eth_string(wei: int) -> str:
    """Integer wei -> decimal ETH string with exactly 18 decimals.

    Example: 1_500_000_000_000_000_000 -> "1.500000000000000000"

    We use integer division/modulo instead of Decimal or float so the result is
    exact for any size of number. (A float has only ~15-17 significant digits,
    which is not enough for 18-decimal ETH amounts.)
    """
    if wei < 0:
        raise ValueError("wei amount cannot be negative")
    whole, frac = divmod(wei, WEI_PER_ETH)
    return f"{whole}.{frac:0{_ETH_DECIMALS}d}"


def unix_to_iso_utc(timestamp: int) -> str:
    """Unix seconds -> 'YYYY-MM-DDTHH:MM:SSZ' (ISO 8601, UTC, no fractions)."""
    return datetime.fromtimestamp(timestamp, tz=timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def explorer_tx_link(base_url: str, tx_hash: str) -> str:
    """Public explorer link stored as `raw_reference` for auditing."""
    if not base_url.endswith("/"):
        base_url += "/"
    return f"{base_url}{tx_hash}"
