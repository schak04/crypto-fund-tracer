"""Configuration: every tunable value lives here, read from environment variables.

Why a dedicated module?
  * One place to look when you wonder "what can I change without editing code?".
  * `Settings` is an immutable dataclass: once built, nothing can change it by
    accident while a request is running.
  * Tests can build a `Settings` directly (see `Settings.for_testing`) instead of
    fiddling with real environment variables.
"""

from __future__ import annotations

import os
from dataclasses import dataclass, replace
from decimal import Decimal
from pathlib import Path

# Folder that contains the `app` package. Data files are resolved relative to
# the project root (one level above `app/`) so `VASP_FILE=app/data/...` works
# no matter which directory the server is started from.
APP_DIR = Path(__file__).resolve().parent
PROJECT_ROOT = APP_DIR.parent

# --- Contract constants (see docs/lld/blockchain-analysis-integration-contract.md)
MIN_DEPTH = 1
MAX_DEPTH = 5
DEFAULT_DEPTH = 3
WEI_PER_ETH = 10**18


def load_env_file(path: Path = PROJECT_ROOT / ".env") -> None:
    """Minimal .env reader: KEY=VALUE lines, '#' comments, never overrides real env vars.

    (We avoid the python-dotenv dependency for something this small.)
    """
    if not path.is_file():
        return
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, _, value = line.partition("=")
        os.environ.setdefault(key.strip(), value.strip().strip('"').strip("'"))


def _env_str(name: str, default: str) -> str:
    value = os.environ.get(name)
    return value.strip() if value is not None and value.strip() != "" else default


def _env_int(name: str, default: int) -> int:
    raw = os.environ.get(name)
    if raw is None or raw.strip() == "":
        return default
    try:
        return int(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer, got {raw!r}") from exc


def _env_float(name: str, default: float) -> float:
    raw = os.environ.get(name)
    if raw is None or raw.strip() == "":
        return default
    try:
        return float(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be a number, got {raw!r}") from exc


def _env_bool(name: str, default: bool) -> bool:
    raw = os.environ.get(name)
    if raw is None or raw.strip() == "":
        return default
    return raw.strip().lower() in {"1", "true", "yes", "on"}


def _env_eth_as_wei(name: str, default: str) -> int:
    """Read an ETH amount like "0.001" and convert it to integer wei.

    We go through `Decimal` (not float) so "0.001" becomes exactly
    1_000_000_000_000_000 wei, with no binary floating point rounding.
    """
    raw = _env_str(name, default)
    try:
        return int(Decimal(raw) * WEI_PER_ETH)
    except Exception as exc:  # decimal.InvalidOperation is a subclass of ArithmeticError
        raise ValueError(f"{name} must be a decimal number, got {raw!r}") from exc


def _resolve_path(raw: str) -> Path:
    path = Path(raw)
    return path if path.is_absolute() else PROJECT_ROOT / path


@dataclass(frozen=True)
class Settings:
    # data source
    provider: str  # "mock" | "etherscan"
    etherscan_api_key: str
    etherscan_base_url: str
    etherscan_chain_id: int
    http_timeout_seconds: float
    min_request_interval_seconds: float
    max_retries: int
    explorer_tx_url: str

    # tracing behaviour
    budget_seconds: float
    max_transfers_per_address: int
    max_expansions: int
    min_value_wei: int
    concurrency: int
    enforce_temporal_order: bool
    check_contracts: bool
    max_contract_checks: int

    # data files
    vasp_file: Path
    mock_scenario_file: Path

    # server
    host: str
    port: int
    log_level: str

    @classmethod
    def from_env(cls) -> "Settings":
        settings = cls(
            provider=_env_str("PROVIDER", "mock").lower(),
            etherscan_api_key=_env_str("ETHERSCAN_API_KEY", ""),
            etherscan_base_url=_env_str("ETHERSCAN_BASE_URL", "https://api.etherscan.io/v2/api"),
            etherscan_chain_id=_env_int("ETHERSCAN_CHAIN_ID", 1),
            http_timeout_seconds=_env_float("HTTP_TIMEOUT_SECONDS", 10.0),
            min_request_interval_seconds=_env_float("MIN_REQUEST_INTERVAL_SECONDS", 0.25),
            max_retries=_env_int("MAX_RETRIES", 2),
            explorer_tx_url=_env_str("EXPLORER_TX_URL", "https://etherscan.io/tx/"),
            budget_seconds=_env_float("BUDGET_SECONDS", 25.0),
            max_transfers_per_address=_env_int("MAX_TRANSFERS_PER_ADDRESS", 10),
            max_expansions=_env_int("MAX_EXPANSIONS", 40),
            min_value_wei=_env_eth_as_wei("MIN_VALUE_ETH", "0.001"),
            concurrency=_env_int("CONCURRENCY", 3),
            enforce_temporal_order=_env_bool("ENFORCE_TEMPORAL_ORDER", True),
            check_contracts=_env_bool("CHECK_CONTRACTS", True),
            max_contract_checks=_env_int("MAX_CONTRACT_CHECKS", 20),
            vasp_file=_resolve_path(_env_str("VASP_FILE", "app/data/vasp_addresses.json")),
            mock_scenario_file=_resolve_path(
                _env_str("MOCK_SCENARIO_FILE", "app/data/demo_scenario.json")
            ),
            host=_env_str("HOST", "0.0.0.0"),
            port=_env_int("PORT", 9090),
            log_level=_env_str("LOG_LEVEL", "INFO").upper(),
        )
        settings.validate()
        return settings

    def validate(self) -> None:
        """Fail at startup (not in the middle of an investigation) on bad config."""
        if self.provider not in {"mock", "etherscan"}:
            raise ValueError(f"PROVIDER must be 'mock' or 'etherscan', got {self.provider!r}")
        if self.provider == "etherscan" and not self.etherscan_api_key:
            raise ValueError("ETHERSCAN_API_KEY is required when PROVIDER=etherscan")
        if self.budget_seconds <= 0:
            raise ValueError("BUDGET_SECONDS must be > 0")
        if self.max_transfers_per_address < 1:
            raise ValueError("MAX_TRANSFERS_PER_ADDRESS must be >= 1")
        if self.max_expansions < 1:
            raise ValueError("MAX_EXPANSIONS must be >= 1")
        if self.concurrency < 1:
            raise ValueError("CONCURRENCY must be >= 1")
        if self.min_value_wei < 0:
            raise ValueError("MIN_VALUE_ETH must be >= 0")

    @classmethod
    def for_testing(cls, **overrides) -> "Settings":
        """Deterministic settings for unit tests: mock provider, no waiting, no env."""
        base = cls(
            provider="mock",
            etherscan_api_key="test-key",
            etherscan_base_url="https://api.etherscan.io/v2/api",
            etherscan_chain_id=1,
            http_timeout_seconds=2.0,
            min_request_interval_seconds=0.0,
            max_retries=1,
            explorer_tx_url="https://etherscan.io/tx/",
            budget_seconds=5.0,
            max_transfers_per_address=10,
            max_expansions=40,
            min_value_wei=10**15,  # 0.001 ETH
            concurrency=3,
            enforce_temporal_order=True,
            check_contracts=True,
            max_contract_checks=20,
            vasp_file=APP_DIR / "data" / "vasp_addresses.json",
            mock_scenario_file=APP_DIR / "data" / "demo_scenario.json",
            host="127.0.0.1",
            port=9090,
            log_level="WARNING",
        )
        return replace(base, **overrides)
