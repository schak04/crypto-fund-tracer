"""The HTTP layer: exposes `POST /analysis` exactly as the integration contract says.

    Go backend  --POST /analysis {wallet_address, max_depth}-->  this service
                <--200 {addresses, transactions, fund_flow, vasp_attribution}--
                <--4xx/5xx {"error": {"code", "message"}}--

This file contains NO tracing logic. It only:
  1. builds the service once at startup (provider + registry + settings),
  2. accepts the request and hands it to AnalysisService,
  3. makes sure EVERY failure leaves as the contract's error envelope.

Point 3 matters: FastAPI's default validation error is HTTP 422 with its own JSON
shape. The Go client only understands {"error": {"code", "message"}}, and the
contract says bad input must be 400 INVALID_REQUEST, so we override the defaults.

Run it:    uvicorn app.main:create_app --factory --port 9090      (or: make run)

`--factory` tells uvicorn "call create_app() to get the app". That way importing
this module (e.g. from tests) never reads environment variables or opens files.
"""

from __future__ import annotations

import logging
from contextlib import asynccontextmanager
from typing import Optional

from fastapi import FastAPI, Request
from fastapi.exceptions import RequestValidationError
from fastapi.responses import JSONResponse
from starlette.exceptions import HTTPException as StarletteHTTPException

from app.config import Settings
from app.errors import AnalysisError
from app.models import AnalysisRequest, AnalysisResponse
from app.providers import build_provider
from app.providers.base import TransferProvider
from app.service import AnalysisService
from app.vasp import VaspRegistry

log = logging.getLogger("app.main")


def create_app(settings: Optional[Settings] = None, provider: Optional[TransferProvider] = None) -> FastAPI:
    """Application factory.

    Tests pass in their own `settings` / `provider`; production uses the defaults
    (read from environment variables, provider chosen by PROVIDER).
    """
    settings = settings or Settings.from_env()
    logging.basicConfig(
        level=getattr(logging, settings.log_level, logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )

    @asynccontextmanager
    async def lifespan(app: FastAPI):
        data_source = provider or build_provider(settings)
        registry = VaspRegistry.from_file(settings.vasp_file)  # fails fast on a bad file
        app.state.service = AnalysisService(data_source, registry, settings)
        log.info(
            "analysis service ready provider=%s vasp_addresses=%d budget=%.0fs",
            settings.provider, len(registry), settings.budget_seconds,
        )
        try:
            yield
        finally:
            await data_source.aclose()

    app = FastAPI(title="Crypto Fund Tracer - Blockchain Analysis", version="1.0.0", lifespan=lifespan)

    # ------------------------------------------------------------------ routes

    @app.post("/analysis", response_model=AnalysisResponse)
    async def analysis(payload: AnalysisRequest, request: Request) -> AnalysisResponse:
        return await request.app.state.service.analyze(payload)

    @app.get("/health")
    async def health() -> dict:
        return {"status": "ok", "provider": settings.provider}

    # ------------------------------------------------- error envelope handlers

    @app.exception_handler(AnalysisError)
    async def handle_analysis_error(_: Request, exc: AnalysisError) -> JSONResponse:
        log.warning("analysis error %s: %s", exc.code, exc.message)
        return JSONResponse(status_code=exc.http_status, content=exc.to_payload())

    @app.exception_handler(RequestValidationError)
    async def handle_validation_error(_: Request, exc: RequestValidationError) -> JSONResponse:
        # Describe the first problem in plain words, e.g. "max_depth: Input should be <= 5".
        first = exc.errors()[0] if exc.errors() else {}
        location = ".".join(str(part) for part in first.get("loc", []) if part != "body")
        reason = str(first.get("msg", "invalid request")).removeprefix("Value error, ")
        message = f"{location}: {reason}" if location else reason
        error = AnalysisError.invalid_request(f"Invalid request payload - {message}")
        return JSONResponse(status_code=error.http_status, content=error.to_payload())

    @app.exception_handler(StarletteHTTPException)
    async def handle_http_exception(_: Request, exc: StarletteHTTPException) -> JSONResponse:
        # 404 unknown path, 405 wrong method, ...: keep the same envelope shape.
        code = "INVALID_REQUEST" if exc.status_code < 500 else "ANALYSIS_FAILED"
        return JSONResponse(
            status_code=exc.status_code,
            content={"error": {"code": code, "message": str(exc.detail)}},
        )

    @app.exception_handler(Exception)
    async def handle_unexpected(_: Request, exc: Exception) -> JSONResponse:
        log.exception("unhandled error")
        error = AnalysisError.analysis_failed("Unexpected internal error.")
        return JSONResponse(status_code=error.http_status, content=error.to_payload())

    return app
