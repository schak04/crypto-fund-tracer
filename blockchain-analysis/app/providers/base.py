"""The provider interface: the ONLY thing the tracer knows about blockchain data.

    tracer  --asks-->  TransferProvider  --implemented by-->  EtherscanProvider
                                                              MockProvider

Why an interface (abstract class)?
  * The MVP uses one data source, but docs/hld/architecture.md says the choice
    was still open. With this seam you can swap Etherscan for Alchemy, a local
    node, or anything else by writing ONE new class. The tracer is untouched.
  * Tests and demos use MockProvider: no network, no API key, deterministic.

Rules every provider must follow (the tracer relies on them):
  1. `get_outgoing_transfers(address)` returns ONLY successful transfers of
     native ETH whose sender is `address`, with value > 0.
  2. All addresses in the returned objects are lowercase.
  3. Failures are raised as AnalysisError (rpc_timeout / rate_limited /
     analysis_failed) so the HTTP layer can report them with the right code.
"""

from __future__ import annotations

from abc import ABC, abstractmethod

from app.domain import Transfer


class TransferProvider(ABC):
    @abstractmethod
    async def get_outgoing_transfers(self, address: str) -> list[Transfer]:
        """All (recent) successful ETH transfers sent BY `address`."""

    @abstractmethod
    async def is_contract(self, address: str) -> bool:
        """True if `address` holds smart-contract code (so it is not a person's wallet)."""

    async def aclose(self) -> None:
        """Release network resources. Default: nothing to release."""
        return None
