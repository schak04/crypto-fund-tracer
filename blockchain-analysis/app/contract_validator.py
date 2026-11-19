"""Self-check: verify our own response against the integration contract.

This is section 9 of docs/lld/blockchain-analysis-integration-contract.md
("Implementation Checklist") turned into code that runs on EVERY response.

Why check our own output?
  The Go backend writes whatever we send straight into PostgreSQL inside one
  database transaction. A response that breaks an invariant (an edge pointing at
  an address that is not in `addresses`, an amount with a comma, a missing
  suspect) could corrupt evidence or fail the whole save. It is far better to
  fail loudly HERE with ANALYSIS_FAILED than to store a wrong graph quietly.
  If this ever fires, it means there is a bug in our code, not in the input.
"""

from __future__ import annotations

import re

from app.errors import AnalysisError
from app.models import AnalysisResponse

_AMOUNT_RE = re.compile(r"^\d+\.\d{18}$")
_TIME_RE = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")
_HASH_RE = re.compile(r"^0x[0-9a-f]{64}$")


def _fail(problem: str) -> AnalysisError:
    return AnalysisError.analysis_failed(f"Response violates the integration contract: {problem}")


def validate_response(response: AnalysisResponse, requested_wallet: str, max_depth: int) -> None:
    # 1. echo of the queried wallet
    if response.wallet_address != requested_wallet:
        raise _fail("wallet_address does not echo the requested wallet")

    # 2. addresses: unique, suspect present exactly once
    seen: set[str] = set()
    for item in response.addresses:
        key = item.address.lower()
        if key in seen:
            raise _fail(f"duplicate address {item.address}")
        seen.add(key)
    suspects = [a for a in response.addresses if a.role == "suspect"]
    if len(suspects) != 1 or suspects[0].address.lower() != requested_wallet.lower():
        raise _fail("the suspect address must appear exactly once with role 'suspect'")

    # 3. every edge endpoint is a known address; amounts/hashes are well formed
    for edge in response.fund_flow:
        for end in (edge.source_address, edge.destination_address):
            if end.lower() not in seen:
                raise _fail(f"fund_flow references {end}, which is not in addresses")
        if not _AMOUNT_RE.match(edge.amount):
            raise _fail(f"edge amount {edge.amount!r} is not an 18-decimal string")
        if not _HASH_RE.match(edge.tx_hash):
            raise _fail(f"edge tx_hash {edge.tx_hash!r} is malformed")

    # 4. transactions: unique hashes, formats, depth range
    tx_hashes: set[str] = set()
    for tx in response.transactions:
        if tx.tx_hash in tx_hashes:
            raise _fail(f"duplicate transaction {tx.tx_hash}")
        tx_hashes.add(tx.tx_hash)
        if not _AMOUNT_RE.match(tx.amount):
            raise _fail(f"transaction amount {tx.amount!r} is not an 18-decimal string")
        if not _TIME_RE.match(tx.tx_time):
            raise _fail(f"tx_time {tx.tx_time!r} is not ISO 8601 UTC")
        if not 1 <= tx.trace_depth <= max_depth:
            raise _fail(f"trace_depth {tx.trace_depth} outside 1..{max_depth}")
        if not tx.raw_reference:
            raise _fail("raw_reference is empty")

    # 5. edges and transactions describe the same set of hashes
    flow_hashes = {e.tx_hash for e in response.fund_flow}
    if flow_hashes != tx_hashes:
        raise _fail("fund_flow and transactions do not reference the same tx_hash set")

    # 6. attribution must point at a destination address in the graph
    attribution = response.vasp_attribution
    if attribution is not None:
        match = [a for a in response.addresses if a.address.lower() == attribution.matched_address.lower()]
        if not match or match[0].role != "destination":
            raise _fail("vasp_attribution.matched_address must be an address with role 'destination'")
