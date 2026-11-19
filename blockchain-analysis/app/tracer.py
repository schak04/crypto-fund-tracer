"""The fund-flow tracer: the heart of the service.

GOAL: starting from the suspect wallet, follow the money outward, hop by hop,
until it reaches a known exchange, runs out, or we hit the depth limit.

ALGORITHM = Breadth-First Search (BFS) over a graph
---------------------------------------------------
Think of wallets as dots and transfers as arrows:

    SUSPECT --5 ETH--> M1 --4.9 ETH--> M2 --4.8 ETH--> DEPOSIT --> BINANCE
       \\--2 ETH--> M3 --1.9 ETH--> TERM

BFS explores "all wallets 1 hop away", then "all wallets 2 hops away", and so
on. That matches the contract's definition of depth exactly:

    depth 1 = transfers SENT BY the suspect
    depth 2 = transfers sent by the receivers of depth-1 transfers
    depth N = transfers sent by the receivers of depth-(N-1) transfers

Each loop iteration of `trace()` below handles ONE depth level.

HALTING RULES (contract section 5) and where the code does them
---------------------------------------------------------------
  1. reached max_depth                -> the `for depth in ...` loop ends
  2. destination is a known exchange  -> VASP nodes are never put in `frontier`
  3. wallet has no outgoing transfers -> `_select` returns [] so it adds nothing
  4. cycles (funds return to a wallet)-> `result.depth_of` remembers every wallet
                                         seen; we record the edge but never
                                         expand the same wallet twice

FORENSIC RULES that keep the graph honest (beyond the contract)
---------------------------------------------------------------
  * TEMPORAL ORDER: money cannot leave a wallet before it arrived. If funds
    reached M1 at 18:30, a transfer M1 made at 17:00 cannot be those funds.
    Following it would draw a false path. We drop such transfers.
  * DUST FILTER: tiny transfers (e.g. 0.00001 ETH "dusting" spam) are noise.
  * FAN-OUT CAP: a wallet with thousands of outgoing transfers would explode the
    graph. We follow only the N largest ("follow the money").
  * EXPANSION CAP: at most `max_expansions` wallets are fetched in total, largest
    incoming amounts first, so the time budget is spent on the biggest flows.

AFTER the search we assign roles (suspect / intermediary / destination / other).
"""

from __future__ import annotations

import asyncio
import logging
from typing import Optional

from app.config import Settings
from app.domain import Edge, TraceResult, Transfer, VaspHit
from app.errors import AnalysisError
from app.formatting import normalize_address
from app.providers.base import TransferProvider
from app.vasp import VaspRegistry

log = logging.getLogger(__name__)


class FundTracer:
    def __init__(self, provider: TransferProvider, registry: VaspRegistry, settings: Settings) -> None:
        self._provider = provider
        self._registry = registry
        self._s = settings

    # ------------------------------------------------------------------ public

    async def trace(self, suspect_address: str, max_depth: int) -> TraceResult:
        suspect = normalize_address(suspect_address)
        result = TraceResult(suspect=suspect)
        result.depth_of[suspect] = 0

        semaphore = asyncio.Semaphore(self._s.concurrency)

        # Bookkeeping per wallet ------------------------------------------------
        # arrival[a]  = earliest unix time funds could have reached wallet a
        #               (None for the suspect: we don't know when the victim paid)
        arrival: dict[str, Optional[int]] = {suspect: None}
        # incoming_wei[a] = total ETH that reached a at the level it was discovered
        incoming_wei: dict[str, int] = {}
        outgoing_count: dict[str, int] = {}
        vasp_nodes: dict[str, VaspHit] = {}

        frontier: list[str] = [suspect]  # wallets whose outgoing transfers we fetch next

        for depth in range(1, max_depth + 1):
            if not frontier:
                break

            # Spend at most `max_expansions` provider lookups in total.
            remaining = self._s.max_expansions - result.expansions
            if remaining <= 0:
                result.hit_expansion_cap = True
                break
            if len(frontier) > remaining:
                result.hit_expansion_cap = True
                frontier = frontier[:remaining]  # frontier is already sorted: biggest first
            result.expansions += len(frontier)

            # 1) Fetch every frontier wallet's history (concurrently, but throttled).
            histories = await self._fetch_all(semaphore, frontier)

            # 2) Turn history into edges, discover new wallets.
            level_arrival: dict[str, int] = {}
            level_incoming: dict[str, int] = {}
            new_nodes: list[str] = []

            for address, transfers in zip(frontier, histories):
                chosen, truncated = self._select(address, transfers, arrival.get(address))
                if truncated:
                    result.truncated_addresses += 1
                for transfer in chosen:
                    result.edges.append(
                        Edge(
                            tx_hash=transfer.tx_hash,
                            source=address,
                            destination=transfer.to_address,
                            value_wei=transfer.value_wei,
                            timestamp=transfer.timestamp,
                            depth=depth,
                        )
                    )
                    outgoing_count[address] = outgoing_count.get(address, 0) + 1

                    destination = transfer.to_address
                    if destination not in result.depth_of:  # first time we see this wallet
                        result.depth_of[destination] = depth
                        new_nodes.append(destination)
                    if result.depth_of[destination] == depth:
                        # Track arrival time/amount only for wallets discovered at THIS level;
                        # wallets seen earlier (cycles) are already expanded or queued.
                        previous = level_arrival.get(destination)
                        level_arrival[destination] = (
                            transfer.timestamp if previous is None else min(previous, transfer.timestamp)
                        )
                        level_incoming[destination] = level_incoming.get(destination, 0) + transfer.value_wei

            # 3) Decide who to expand next. Known exchanges stop the search (halting rule 2).
            candidates: list[str] = []
            for node in new_nodes:
                arrival[node] = level_arrival[node]
                incoming_wei[node] = level_incoming[node]
                entry = self._registry.lookup(node)
                if entry is not None:
                    vasp_nodes[node] = VaspHit(entry=entry, depth=depth, incoming_wei=level_incoming[node])
                else:
                    candidates.append(node)

            # Biggest incoming amount first, address as a stable tie-breaker.
            candidates.sort(key=lambda a: (-incoming_wei[a], a))
            frontier = candidates

        result.vasp_hits = list(vasp_nodes.values())
        await self._assign_roles(result, outgoing_count, vasp_nodes, semaphore)
        log.info(
            "trace finished suspect=%s depth<=%d wallets=%d edges=%d expansions=%d truncated=%d cap_hit=%s",
            suspect, max_depth, len(result.depth_of), len(result.edges),
            result.expansions, result.truncated_addresses, result.hit_expansion_cap,
        )
        return result

    # ----------------------------------------------------------------- helpers

    async def _fetch_all(self, semaphore: asyncio.Semaphore, addresses: list[str]) -> list[list[Transfer]]:
        async def fetch(address: str) -> list[Transfer]:
            async with semaphore:
                return await self._provider.get_outgoing_transfers(address)

        outcomes = await asyncio.gather(*(fetch(a) for a in addresses), return_exceptions=True)
        for outcome in outcomes:  # report the first failure, in a deterministic order
            if isinstance(outcome, BaseException):
                raise outcome
        return list(outcomes)  # type: ignore[arg-type]

    def _select(
        self, address: str, transfers: list[Transfer], since: Optional[int]
    ) -> tuple[list[Transfer], bool]:
        """Pick which of a wallet's outgoing transfers are worth following.

        Returns (chosen_transfers, was_truncated_by_the_cap).
        """
        eligible: list[Transfer] = []
        minimum = max(1, self._s.min_value_wei)
        for transfer in transfers:
            if transfer.from_address != address:
                continue  # not sent by this wallet (defensive; providers already filter)
            if transfer.to_address == address:
                continue  # self-transfer: no money actually moved
            if transfer.value_wei < minimum:
                continue  # dust
            if self._s.enforce_temporal_order and since is not None and transfer.timestamp < since:
                continue  # left the wallet BEFORE the traced funds arrived
            eligible.append(transfer)

        # Largest first; ties broken by time then hash so results are reproducible.
        eligible.sort(key=lambda t: (-t.value_wei, t.timestamp, t.tx_hash, t.to_address))
        cap = self._s.max_transfers_per_address
        return eligible[:cap], len(eligible) > cap

    async def _assign_roles(
        self,
        result: TraceResult,
        outgoing_count: dict[str, int],
        vasp_nodes: dict[str, VaspHit],
        semaphore: asyncio.Semaphore,
    ) -> None:
        """Label every wallet with one of: suspect, intermediary, destination, other.

        suspect       the wallet that was reported
        destination   a known exchange, OR a wallet where the trace stopped
                      (no outgoing transfers found, or we reached max_depth)
        intermediary  a pass-through wallet: funds arrived AND we saw funds leave
        other         a smart contract that the trace ended in (not a person's wallet)
        """
        for address in result.depth_of:
            if address == result.suspect:
                result.roles[address] = "suspect"
            elif address in vasp_nodes:
                result.roles[address] = "destination"
            elif outgoing_count.get(address, 0) > 0:
                result.roles[address] = "intermediary"
            else:
                result.roles[address] = "destination"

        if not self._s.check_contracts:
            return

        # Which end-of-trace wallets are actually smart contracts? Ask the provider,
        # but only for a bounded number of them (each check is one API call).
        end_points = [
            a for a, role in result.roles.items() if role == "destination" and a not in vasp_nodes
        ]
        end_points.sort(key=lambda a: (result.depth_of[a], a))
        end_points = end_points[: self._s.max_contract_checks]
        if not end_points:
            return

        async def check(address: str) -> bool:
            async with semaphore:
                try:
                    return await self._provider.is_contract(address)
                except AnalysisError as exc:
                    # Not fatal: the graph is still correct, we just can't refine this label.
                    log.warning("contract check failed for %s: %s", address, exc.message)
                    return False

        flags = await asyncio.gather(*(check(a) for a in end_points))
        for address, is_contract in zip(end_points, flags):
            if is_contract:
                result.roles[address] = "other"
