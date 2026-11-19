"""Shared test helpers: the demo blockchain's well-known addresses and factories."""

import asyncio

import pytest

from app.config import Settings
from app.providers.mock import MockProvider
from app.service import AnalysisService
from app.tracer import FundTracer
from app.vasp import VaspRegistry

# Addresses used by app/data/demo_scenario.json
SUSPECT = "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"
SUSPECT_CHECKSUM = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"
M1 = "0x1111111111111111111111111111111111111101"
M2 = "0x2222222222222222222222222222222222222202"
M3 = "0x3333333333333333333333333333333333333303"
DEP = "0x4444444444444444444444444444444444444404"
DUST = "0x5555555555555555555555555555555555555505"
CONTRACT = "0x7777777777777777777777777777777777777707"
TERM = "0x8888888888888888888888888888888888888808"
BINANCE_14 = "0x28c6c06298d514db089934071355e5743bf21d60"
SIMPLE_SUSPECT = "0x742d35cc6634c0532925a3b844bc454e4438f44e"
ERR_TIMEOUT = "0x" + "b" * 39 + "1"
ERR_RATE = "0x" + "b" * 39 + "2"
ERR_FAILED = "0x" + "b" * 39 + "3"


def run(coro):
    return asyncio.run(coro)


@pytest.fixture
def settings():
    return Settings.for_testing()


@pytest.fixture
def make_tracer():
    def factory(**overrides):
        s = Settings.for_testing(**overrides)
        provider = MockProvider.from_file(s.mock_scenario_file)
        return FundTracer(provider, VaspRegistry.from_file(s.vasp_file), s), provider

    return factory


@pytest.fixture
def make_service():
    def factory(provider=None, **overrides):
        s = Settings.for_testing(**overrides)
        provider = provider or MockProvider.from_file(s.mock_scenario_file)
        return AnalysisService(provider, VaspRegistry.from_file(s.vasp_file), s)

    return factory
