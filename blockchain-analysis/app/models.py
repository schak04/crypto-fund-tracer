"""Pydantic models = the integration contract expressed as code.

These classes mirror `internal/analysis/models.go` in the Go backend field by
field. If the JSON produced here does not match, the Go side fails to decode it.

Request  : AnalysisRequest              (POST /analysis body)
Response : AnalysisResponse and its items (HTTP 200 body)
Errors   : see app/errors.py
"""

from __future__ import annotations

import re
from typing import Literal, Optional

from pydantic import BaseModel, ConfigDict, Field, StrictInt, field_validator

from app.config import DEFAULT_DEPTH, MAX_DEPTH, MIN_DEPTH

# Roles / statuses allowed by the contract (and by CHECK constraints in the DB).
Role = Literal["suspect", "intermediary", "destination", "other"]
AttributionStatus = Literal["known", "probable", "unverified"]

# 0x + 40 hex characters. Checksum (EIP-55) is the Go backend's job; here we only
# make sure the string is structurally an EVM address.
_ADDRESS_RE = re.compile(r"^0[xX][0-9a-fA-F]{40}$")


class AnalysisRequest(BaseModel):
    # Ignore unknown fields so the backend can add new ones without breaking us.
    model_config = ConfigDict(extra="ignore")

    wallet_address: str
    # StrictInt: reject "3" or 3.5 or true; the contract says "integer".
    max_depth: StrictInt = Field(default=DEFAULT_DEPTH, ge=MIN_DEPTH, le=MAX_DEPTH)

    @field_validator("wallet_address")
    @classmethod
    def _validate_wallet_address(cls, value: str) -> str:
        value = value.strip()
        if not _ADDRESS_RE.match(value):
            raise ValueError("wallet_address must be 42 characters: '0x' followed by 40 hex digits")
        return value


class AddressItem(BaseModel):
    address: str
    role: Role


class TransactionItem(BaseModel):
    tx_hash: str
    tx_time: str  # ISO 8601 UTC, e.g. 2026-09-16T18:30:00Z (string, not datetime)
    amount: str  # decimal string with 18 decimals, never a float
    trace_depth: int
    raw_reference: str


class FlowEdgeItem(BaseModel):
    tx_hash: str
    source_address: str
    destination_address: str
    amount: str


class VASPAttributionItem(BaseModel):
    name: str
    type: str
    matched_address: str
    attribution_status: AttributionStatus


class AnalysisResponse(BaseModel):
    wallet_address: str
    addresses: list[AddressItem]
    transactions: list[TransactionItem]
    fund_flow: list[FlowEdgeItem]
    # Must serialise as JSON `null` (not be omitted) when nothing was attributed.
    vasp_attribution: Optional[VASPAttributionItem] = None
