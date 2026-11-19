"""AnalysisService: glues the pieces together for one request.

    request --> FundTracer.trace --> build_response --> validate_response --> response
                 (BFS over chain)    (contract shape)     (self-check)

It also owns the TIME BUDGET and the "catch-all" error handling:

  * Time budget: the Go backend abandons the call after 30 s. If we are going to
    run out of time we want to say so with a proper error envelope, not leave the
    backend with a broken connection. So the whole trace runs under a deadline
    (BUDGET_SECONDS, default 25 s).

    Design decision: on timeout we FAIL instead of returning a partial graph.
    The contract has no "incomplete" flag, so the backend would save a partial
    graph as a normal completed investigation and an investigator could
    mistake it for the full picture. A clear failure ("retry with a smaller
    max_depth") is safer for evidence.

  * Anything unexpected (a bug) becomes ANALYSIS_FAILED with a safe message;
    the full stack trace goes to the log, not to the client.
"""

from __future__ import annotations

import asyncio
import logging
import time

from app.builder import build_response
from app.config import Settings
from app.contract_validator import validate_response
from app.errors import AnalysisError
from app.models import AnalysisRequest, AnalysisResponse
from app.providers.base import TransferProvider
from app.tracer import FundTracer
from app.vasp import VaspRegistry

log =logging.getLogger(__name__)


class AnalysisService:
    def __init__(self, provider: TransferProvider, registry: VaspRegistry, settings: Settings) -> None:
        self._settings = settings
        self._tracer = FundTracer(provider, registry, settings)

    async def analyze(self, request: AnalysisRequest) -> AnalysisResponse:
        started = time.monotonic()
        wallet = request.wallet_address  # already stripped by the request model
        try:
            try:
                async with asyncio.timeout(self._settings.budget_seconds):
                    result = await self._tracer.trace(wallet, request.max_depth)
            except TimeoutError as exc:
                raise AnalysisError.analysis_failed(
                    f"Analysis exceeded the {self._settings.budget_seconds:g}s time budget "
                    f"at max_depth={request.max_depth}; retry with a smaller max_depth."
                ) from exc

            response = build_response(result, wallet, self._settings)
            validate_response(response, wallet, request.max_depth)
        except AnalysisError:
            raise
        except Exception as exc:  # a bug: do not leak internals to the caller
            log.exception("unexpected failure while analysing %s", wallet)
            raise AnalysisError.analysis_failed(
                f"Unexpected internal error while tracing funds ({exc.__class__.__name__})."
            ) from exc

        log.info(
            "analysis ok wallet=%s depth=%d addresses=%d transactions=%d attribution=%s seconds=%.2f",
            wallet, request.max_depth, len(response.addresses), len(response.transactions),
            response.vasp_attribution.name if response.vasp_attribution else None,
            time.monotonic() - started,
        )
        return response
