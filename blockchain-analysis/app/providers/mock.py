"""MockProvider: a fake blockchain loaded from a JSON file.

Use it to:
  * develop and test the tracer without internet or an API key,
  * run the whole system (Go backend + this service) end to end in a demo,
  * reproduce edge cases on demand (cycles, dust, failed txs, contracts, upstream errors).

Scenario file format (see app/data/demo_scenario.json):

    {
      "transfers": [ {tx_hash, from, to, value_eth, timestamp, kind?, failed?, label?}, ... ],
      "contracts": [ "0x...", ... ],            # addresses that are smart contracts
      "errors":    { "0x...": "RPC_TIMEOUT" }   # asking for this address raises that error
    }
"""

from __future__ import annotations

import json
from decimal import Decimal
from pathlib import Path
from typing import Any

from app.config import WEI_PER_ETH
from app.domain import Transfer
from app.errors import (
    CODE_ANALYSIS_FAILED,
    CODE_RATE_LIMITED,
    CODE_RPC_TIMEOUT,
    AnalysisError,
)
from app.formatting import normalize_address
from app.providers.base import TransferProvider

_ERROR_FACTORY = {
    CODE_RPC_TIMEOUT: AnalysisError.rpc_timeout,
    CODE_RATE_LIMITED: AnalysisError.rate_limited,
    CODE_ANALYSIS_FAILED: AnalysisError.analysis_failed,
}


class MockProvider(TransferProvider):
    def __init__(self, scenario: dict[str, Any]) -> None:
        self._by_sender: dict[str, list[Transfer]] = {}
        for row in scenario.get("transfers", []):
            if row.get("failed"):
                continue  # a reverted transaction moved no money: providers drop these
            value_wei = int(Decimal(str(row["value_eth"])) * WEI_PER_ETH)
            if value_wei <= 0:
                continue
            transfer = Transfer(
                tx_hash=row["tx_hash"].lower(),
                from_address=normalize_address(row["from"]),
                to_address=normalize_address(row["to"]),
                value_wei=value_wei,
                timestamp=int(row["timestamp"]),
                kind=row.get("kind", "normal"),
            )
            self._by_sender.setdefault(transfer.from_address, []).append(transfer)

        self._contracts = {normalize_address(a) for a in scenario.get("contracts", [])}
        self._errors = {normalize_address(a): code for a, code in scenario.get("errors", {}).items()}
        self.calls: list[str] = []  # handy in tests: which addresses were fetched?

    @classmethod
    def from_file(cls, path: Path) -> "MockProvider":
        with open(path, encoding="utf-8") as handle:
            return cls(json.load(handle))

    def _maybe_fail(self, address: str) -> None:
        code = self._errors.get(address)
        if code:
            factory = _ERROR_FACTORY.get(code, AnalysisError.analysis_failed)
            raise factory(f"Mock provider simulated {code} for {address}")

    async def get_outgoing_transfers(self, address: str) -> list[Transfer]:
        address = normalize_address(address)
        self.calls.append(address)
        self._maybe_fail(address)
        return list(self._by_sender.get(address, []))

    async def is_contract(self, address: str) -> bool:
        return normalize_address(address) in self._contracts
