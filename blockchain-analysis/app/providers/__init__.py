"""Provider factory: pick the data source named by PROVIDER."""

from __future__ import annotations

from app.config import Settings
from app.providers.base import TransferProvider
from app.providers.etherscan import EtherscanProvider
from app.providers.mock import MockProvider


def build_provider(settings: Settings) -> TransferProvider:
    if settings.provider == "mock":
        return MockProvider.from_file(settings.mock_scenario_file)
    if settings.provider == "etherscan":
        return EtherscanProvider(settings)
    raise ValueError(f"Unknown provider {settings.provider!r}")  # Settings.validate prevents this


__all__ = ["TransferProvider", "MockProvider", "EtherscanProvider", "build_provider"]
