"""Internal (non-contract) data structures used while tracing.

Contract models (app/models.py) are what we send OUT over HTTP.
Domain objects (this file) are what the algorithm works with INSIDE.
Keeping them separate means we can change the algorithm without touching the
contract, and the contract without touching the algorithm.

Amounts here are always integer wei (1 ETH = 10**18 wei). Integers are exact;
floats are not. We only turn wei into a decimal string at the very end.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal

TransferKind = Literal["normal", "internal"]


@dataclass(frozen=True)
class Transfer:
    """One movement of native ETH from one address to another.

    A single on-chain transaction can contain several transfers (a "normal"
    one plus "internal" ones created by smart contracts), so several Transfers
    may share the same `tx_hash`.
    """

    tx_hash: str  # lowercase 0x...
    from_address: str  # lowercase
    to_address: str  # lowercase
    value_wei: int
    timestamp: int  # unix seconds (block time)
    kind: TransferKind = "normal"


@dataclass(frozen=True)
class VaspEntry:
    """A known exchange/VASP address from the registry (app/vasp.py)."""

    name: str
    type: str
    attribution_status: str  # known | probable | unverified
    address: str  # lowercase
    label: str = ""  # e.g. "Binance 14" (informational)


@dataclass(frozen=True)
class Edge:
    """A traced transfer plus the hop distance at which we found it."""

    tx_hash: str
    source: str
    destination: str
    value_wei: int
    timestamp: int
    depth: int  # 1 = sent directly by the suspect


@dataclass
class VaspHit:
    """A traced destination that matched the registry."""

    entry: VaspEntry
    depth: int
    incoming_wei: int


@dataclass
class TraceResult:
    """Everything the tracer learned, before it is shaped into the contract."""

    suspect: str  # normalised (lowercase)
    depth_of: dict[str, int] = field(default_factory=dict)  # address -> hop distance
    edges: list[Edge] = field(default_factory=list)
    roles: dict[str, str] = field(default_factory=dict)  # address -> role
    vasp_hits: list[VaspHit] = field(default_factory=list)
    expansions: int = 0  # how many addresses we fetched history for
    truncated_addresses: int = 0  # addresses where the top-N cap dropped transfers
    hit_expansion_cap: bool = False
