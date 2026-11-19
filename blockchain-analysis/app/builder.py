"""Builder: converts the tracer's internal TraceResult into the contract response.

The tracer thinks in wei integers, lowercase addresses and unix timestamps.
The contract wants decimal strings, ISO timestamps and unique lists. This module
is the translation layer, and it also makes the output DETERMINISTIC (stable
ordering) so the same investigation always produces byte-identical JSON, which
is important for audit trails and for testing.

Three things worth understanding:

1. ONE TRANSACTION CAN BE SEVERAL TRANSFERS.
   A smart contract may split one transaction into multiple internal transfers.
   The database stores `transaction` once per tx_hash, so we aggregate:
       transactions[] : one item per tx_hash  (amount = sum, depth = smallest)
       fund_flow[]    : one item per (tx_hash, source, destination), amounts summed

2. ADDRESS SPELLING.
   The Go backend stores addresses as TEXT and case-sensitively, and it saved the
   suspect exactly as the investigator typed it (maybe EIP-55 mixed case). If we
   returned a lowercase suspect, the database would end up with TWO rows for the
   same wallet. So the suspect keeps the exact spelling we received; every other
   address is lowercase.

3. ONLY ONE vasp_attribution.
   The contract allows a single attribution object. If several exchange wallets
   were reached we pick the best by: most trustworthy status, then closest to the
   suspect, then largest amount received, then address (tie-break).
"""

from __future__ import annotations

from typing import Optional

from app.config import Settings
from app.domain import TraceResult, VaspHit
from app.formatting import explorer_tx_link, unix_to_iso_utc, wei_to_eth_string
from app.models import (
    AddressItem,
    AnalysisResponse,
    FlowEdgeItem,
    TransactionItem,
    VASPAttributionItem,
)
from app.vasp import STATUS_RANK


def choose_attribution(hits: list[VaspHit]) -> Optional[VaspHit]:
    if not hits:
        return None
    return min(
        hits,
        key=lambda h: (-STATUS_RANK.get(h.entry.attribution_status, 0), h.depth, -h.incoming_wei, h.entry.address),
    )


def build_response(result: TraceResult, submitted_wallet: str, settings: Settings) -> AnalysisResponse:
    def display(address: str) -> str:
        return submitted_wallet if address == result.suspect else address

    # --- addresses: suspect first, then by hop distance, then alphabetically
    ordered = sorted(result.depth_of, key=lambda a: (result.depth_of[a], a))
    addresses = [AddressItem(address=display(a), role=result.roles[a]) for a in ordered]  # type: ignore[arg-type]

    # --- fund_flow: merge transfers that share (tx_hash, source, destination)
    merged: dict[tuple[str, str, str], dict] = {}
    for edge in result.edges:
        key = (edge.tx_hash, edge.source, edge.destination)
        slot = merged.setdefault(key, {"wei": 0, "depth": edge.depth, "time": edge.timestamp})
        slot["wei"] += edge.value_wei
        slot["depth"] = min(slot["depth"], edge.depth)
        slot["time"] = min(slot["time"], edge.timestamp)

    flow_keys = sorted(merged, key=lambda k: (merged[k]["depth"], k[0], k[1], k[2]))
    fund_flow = [
        FlowEdgeItem(
            tx_hash=tx_hash,
            source_address=display(source),
            destination_address=display(destination),
            amount=wei_to_eth_string(merged[(tx_hash, source, destination)]["wei"]),
        )
        for (tx_hash, source, destination) in flow_keys
    ]

    # --- transactions: one entry per tx_hash
    per_tx: dict[str, dict] = {}
    for (tx_hash, _source, _dest), slot in merged.items():
        tx = per_tx.setdefault(tx_hash, {"wei": 0, "depth": slot["depth"], "time": slot["time"]})
        tx["wei"] += slot["wei"]
        tx["depth"] = min(tx["depth"], slot["depth"])
        tx["time"] = min(tx["time"], slot["time"])

    tx_hashes = sorted(per_tx, key=lambda h: (per_tx[h]["depth"], per_tx[h]["time"], h))
    transactions = [
        TransactionItem(
            tx_hash=tx_hash,
            tx_time=unix_to_iso_utc(per_tx[tx_hash]["time"]),
            amount=wei_to_eth_string(per_tx[tx_hash]["wei"]),
            trace_depth=per_tx[tx_hash]["depth"],
            raw_reference=explorer_tx_link(settings.explorer_tx_url, tx_hash),
        )
        for tx_hash in tx_hashes
    ]

    # --- attribution
    best = choose_attribution(result.vasp_hits)
    attribution = (
        VASPAttributionItem(
            name=best.entry.name,
            type=best.entry.type,
            matched_address=display(best.entry.address),
            attribution_status=best.entry.attribution_status,  # type: ignore[arg-type]
        )
        if best is not None
        else None
    )

    return AnalysisResponse(
        wallet_address=submitted_wallet,
        addresses=addresses,
        transactions=transactions,
        fund_flow=fund_flow,
        vasp_attribution=attribution,
    )
