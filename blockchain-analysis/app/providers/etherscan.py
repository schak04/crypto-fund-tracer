"""EtherscanProvider: real Ethereum data through the Etherscan API (V2).

Which Etherscan endpoints do we use?
  module=account&action=txlist          "normal" transactions (a wallet sends ETH)
  module=account&action=txlistinternal  "internal" transfers (a smart contract forwards ETH)
  module=proxy&action=eth_getCode       is this address a smart contract?

Why both txlist and txlistinternal? If a victim's funds go into a forwarding
contract (very common in laundering), the next hop is an INTERNAL transfer and
a normal-transaction list alone would miss it.

Etherscan answers like this (important to parse correctly):
  success        {"status":"1","message":"OK","result":[ ...rows... ]}
  no data        {"status":"0","message":"No transactions found","result":[]}
  rate limit     {"status":"0","message":"NOTOK","result":"Max rate limit reached"}
  bad key        {"status":"0","message":"NOTOK","result":"Invalid API Key"}
  proxy module   {"jsonrpc":"2.0","id":1,"result":"0x..."}   (no "status" field!)

Known limitation (documented on purpose): we read the most recent
PAGE_SIZE transactions per address. A very busy wallet has more history than
that; we trace its newest activity.
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any, Optional

import httpx

from app.config import Settings
from app.domain import Transfer
from app.errors import AnalysisError
from app.formatting import normalize_address
from app.providers.base import TransferProvider
from app.providers.ratelimit import RateLimiter

log = logging.getLogger(__name__)

PAGE_SIZE = 1000


class EtherscanProvider(TransferProvider):
    def __init__(self, settings: Settings, client: Optional[httpx.AsyncClient] = None) -> None:
        self._s = settings
        # A client can be injected so tests can plug in httpx.MockTransport.
        self._client = client or httpx.AsyncClient(timeout=settings.http_timeout_seconds)
        self._owns_client = client is None
        self._limiter = RateLimiter(settings.min_request_interval_seconds)

    async def aclose(self) -> None:
        if self._owns_client:
            await self._client.aclose()

    # ------------------------------------------------------------------ public

    async def get_outgoing_transfers(self, address: str) -> list[Transfer]:
        address = normalize_address(address)
        transfers: list[Transfer] = []
        for action, kind in (("txlist", "normal"), ("txlistinternal", "internal")):
            rows = await self._call(
                {
                    "module": "account",
                    "action": action,
                    "address": address,
                    "startblock": 0,
                    "endblock": 99999999,
                    "page": 1,
                    "offset": PAGE_SIZE,
                    "sort": "desc",  # newest first, so the page holds the recent activity
                }
            )
            if not isinstance(rows, list):
                raise AnalysisError.analysis_failed(
                    f"Unexpected Etherscan response for {action}: expected a list of transactions."
                )
            for row in rows:
                transfer = self._parse_row(row, kind, address)
                if transfer is not None:
                    transfers.append(transfer)
        return transfers

    async def is_contract(self, address: str) -> bool:
        code = await self._call(
            {"module": "proxy", "action": "eth_getCode", "address": normalize_address(address), "tag": "latest"}
        )
        if not isinstance(code, str):
            raise AnalysisError.analysis_failed("Unexpected Etherscan response for eth_getCode.")
        code = code.lower()
        if code in ("", "0x", "0x0"):
            return False  # no code -> a normal wallet (EOA)
        # EIP-7702: a normal wallet can "delegate" to a contract; its code is the
        # marker 0xef0100 + 20-byte address. It is still a person's wallet.
        if code.startswith("0xef0100"):
            return False
        return True

    # ----------------------------------------------------------------- parsing

    @staticmethod
    def _parse_row(row: dict[str, Any], kind: str, address: str) -> Optional[Transfer]:
        """Turn one Etherscan row into a Transfer, or None if it is not relevant.

        Dropped on purpose:
          * failed/reverted transactions (money did not move)
          * rows not sent BY `address` (the endpoint also returns incoming rows)
          * zero-value rows (plain contract calls)
          * rows with no recipient at all
        """
        try:
            if str(row.get("isError", "0")) == "1" or str(row.get("txreceipt_status", "1")) == "0":
                return None
            sender = normalize_address(str(row["from"]))
            if sender != address:
                return None
            value = int(row["value"])
            if value <= 0:
                return None
            # Contract creation: `to` is empty and the new contract is in `contractAddress`.
            recipient_raw = str(row.get("to") or row.get("contractAddress") or "")
            if not recipient_raw:
                return None
            return Transfer(
                tx_hash=str(row["hash"]).lower(),
                from_address=sender,
                to_address=normalize_address(recipient_raw),
                value_wei=value,
                timestamp=int(row["timeStamp"]),
                kind=kind,  # type: ignore[arg-type]
            )
        except (KeyError, ValueError, TypeError) as exc:
            raise AnalysisError.analysis_failed(
                f"Failed to parse Etherscan transaction row ({exc.__class__.__name__}: {exc})."
            ) from exc

    # --------------------------------------------------------------- transport

    async def _call(self, params: dict[str, Any]) -> Any:
        """One Etherscan call with rate limiting, retry-on-rate-limit and error mapping."""
        query = {"chainid": self._s.etherscan_chain_id, "apikey": self._s.etherscan_api_key, **params}
        attempts = self._s.max_retries + 1
        last_rate_message = ""

        for attempt in range(attempts):
            await self._limiter.wait()
            try:
                response = await self._client.get(self._s.etherscan_base_url, params=query)
            except httpx.TimeoutException as exc:
                raise AnalysisError.rpc_timeout(
                    f"Etherscan request timed out after {self._s.http_timeout_seconds:g} seconds."
                ) from exc
            except httpx.HTTPError as exc:
                raise AnalysisError.rpc_timeout(
                    f"Could not reach Etherscan ({exc.__class__.__name__})."
                ) from exc

            if response.status_code == 429:
                last_rate_message = "HTTP 429 Too Many Requests"
                await self._backoff(attempt)
                continue
            if response.status_code >= 500:
                raise AnalysisError.rpc_timeout(f"Etherscan returned HTTP {response.status_code}.")
            if response.status_code != 200:
                raise AnalysisError.analysis_failed(f"Etherscan returned HTTP {response.status_code}.")

            try:
                data = response.json()
            except ValueError as exc:
                raise AnalysisError.analysis_failed("Etherscan returned a non-JSON response.") from exc
            if not isinstance(data, dict):
                raise AnalysisError.analysis_failed("Etherscan returned an unexpected JSON shape.")

            # JSON-RPC style answer from the `proxy` module (no "status" key).
            if "status" not in data and "result" in data:
                return data["result"]

            result = data.get("result")
            if str(data.get("status")) == "1":
                return result

            message = str(data.get("message", ""))
            if isinstance(result, list) and not result:
                return []  # status "0" + empty list == "No transactions found"

            text = f"{message} {result}".strip()
            lowered = text.lower()
            if "rate limit" in lowered:
                last_rate_message = text
                await self._backoff(attempt)
                continue
            if "api key" in lowered:
                raise AnalysisError.analysis_failed(
                    "Etherscan rejected the API key (check ETHERSCAN_API_KEY)."
                )
            raise AnalysisError.analysis_failed(f"Etherscan error: {text}")

        raise AnalysisError.rate_limited(
            f"Etherscan rate limit still exceeded after {attempts} attempts: {last_rate_message}"
        )

    async def _backoff(self, attempt: int) -> None:
        # 0 when the limiter is disabled (tests); ~1s, 2s, ... with the default 0.25s interval.
        await asyncio.sleep(4 * self._s.min_request_interval_seconds * (attempt + 1))
