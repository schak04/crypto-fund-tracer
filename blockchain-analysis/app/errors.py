"""Error types that map 1:1 onto the integration contract (section 8).

The Go backend only understands this envelope:

    {"error": {"code": "<CODE>", "message": "<human readable>"}}

and these four (status, code) pairs:

    400 INVALID_REQUEST   bad payload / bad address / depth out of range
    500 ANALYSIS_FAILED   our own bug, parse failure, contract violation
    502 RPC_TIMEOUT       upstream node/explorer timed out or refused
    502 RATE_LIMITED      upstream provider rate limit exceeded

Everywhere else in the code we simply `raise AnalysisError.rpc_timeout("...")`
and the FastAPI layer (app/main.py) turns it into the right HTTP response.
That keeps HTTP knowledge out of the tracing logic.
"""

from __future__ import annotations

CODE_INVALID_REQUEST = "INVALID_REQUEST"
CODE_ANALYSIS_FAILED = "ANALYSIS_FAILED"
CODE_RPC_TIMEOUT = "RPC_TIMEOUT"
CODE_RATE_LIMITED = "RATE_LIMITED"


class AnalysisError(Exception):
    """Any failure that must be reported to the Go backend in the error envelope."""

    def __init__(self, code: str, message: str, http_status: int) -> None:
        super().__init__(message)
        self.code = code
        self.message = message
        self.http_status = http_status

    def to_payload(self) -> dict:
        return {"error": {"code": self.code, "message": self.message}}

    # Named constructors: they make call sites read like the contract table.
    @classmethod
    def invalid_request(cls, message: str) -> "AnalysisError":
        return cls(CODE_INVALID_REQUEST, message, 400)

    @classmethod
    def analysis_failed(cls, message: str) -> "AnalysisError":
        return cls(CODE_ANALYSIS_FAILED, message, 500)

    @classmethod
    def rpc_timeout(cls, message: str) -> "AnalysisError":
        return cls(CODE_RPC_TIMEOUT, message, 502)

    @classmethod
    def rate_limited(cls, message: str) -> "AnalysisError":
        return cls(CODE_RATE_LIMITED, message, 502)

    def __repr__(self) -> str:  # helpful in test failures
        return f"AnalysisError({self.code!r}, {self.message!r}, {self.http_status})"
