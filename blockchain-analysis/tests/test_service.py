"""Tests of the response shaping (builder), the self-check (validator) and the service."""

import asyncio
import json

import pytest

from app.builder import choose_attribution
from app.contract_validator import validate_response
from app.domain import VaspEntry, VaspHit
from app.errors import CODE_ANALYSIS_FAILED, AnalysisError
from app.models import AnalysisRequest, AnalysisResponse
from app.providers.mock import MockProvider
from tests.conftest import BINANCE_14, SIMPLE_SUSPECT, SUSPECT, SUSPECT_CHECKSUM, run

T0 = 1_789_583_400


def analyze(service, wallet, depth=5):
    return run(service.analyze(AnalysisRequest(wallet_address=wallet, max_depth=depth)))


def test_full_response_matches_contract_shape(make_service):
    response = analyze(make_service(), SUSPECT)

    assert response.wallet_address == SUSPECT
    assert len(response.addresses) == 8
    assert len(response.transactions) == 8
    assert len(response.fund_flow) == 8
    assert response.addresses[0].model_dump() == {"address": SUSPECT, "role": "suspect"}

    first_tx = response.transactions[0]
    assert first_tx.trace_depth == 1
    assert first_tx.amount == "5.000000000000000000"
    assert first_tx.tx_time == "2026-09-16T18:30:00Z"
    assert first_tx.raw_reference == f"https://etherscan.io/tx/{first_tx.tx_hash}"

    assert response.vasp_attribution.model_dump() == {
        "name": "Binance", "type": "exchange", "matched_address": BINANCE_14, "attribution_status": "known",
    }


def test_json_has_all_top_level_keys_and_explicit_null_attribution(make_service):
    payload = json.loads(analyze(make_service(), SUSPECT, depth=3).model_dump_json())
    assert set(payload) == {"wallet_address", "addresses", "transactions", "fund_flow", "vasp_attribution"}
    assert payload["vasp_attribution"] is None  # present and null, not omitted


def test_isolated_wallet_gives_valid_empty_collections(make_service):
    payload = json.loads(analyze(make_service(), "0x" + "9" * 40).model_dump_json())
    assert payload["transactions"] == [] and payload["fund_flow"] == []
    assert len(payload["addresses"]) == 1 and payload["vasp_attribution"] is None


def test_suspect_keeps_submitted_spelling_everywhere(make_service):
    response = analyze(make_service(), SUSPECT_CHECKSUM, depth=1)
    assert response.wallet_address == SUSPECT_CHECKSUM
    assert response.addresses[0].address == SUSPECT_CHECKSUM
    assert {e.source_address for e in response.fund_flow} == {SUSPECT_CHECKSUM}
    # every other address is lowercase
    assert all(a.address == a.address.lower() for a in response.addresses[1:])


def test_direct_exchange_case_from_the_readme(make_service):
    response = analyze(make_service(), SIMPLE_SUSPECT, depth=3)
    assert [a.role for a in response.addresses] == ["suspect", "destination"]
    assert response.fund_flow[0].amount == "1.500000000000000000"
    assert response.vasp_attribution.matched_address == BINANCE_14


def test_output_is_deterministic(make_service):
    a = analyze(make_service(), SUSPECT).model_dump_json()
    b = analyze(make_service(), SUSPECT).model_dump_json()
    assert a == b


def test_multiple_transfers_in_one_transaction_are_aggregated(make_service):
    """A contract can split one tx into several internal transfers: the DB wants one tx row."""
    h = "0x" + "ab" * 32
    a, b, c = "0x" + "a" * 40, "0x" + "b" * 40, "0x" + "c" * 40
    scenario = {"transfers": [
        {"tx_hash": h, "from": a, "to": b, "value_eth": "1.0", "timestamp": T0},
        {"tx_hash": h, "from": a, "to": b, "value_eth": "0.5", "timestamp": T0, "kind": "internal"},
        {"tx_hash": h, "from": a, "to": c, "value_eth": "2.0", "timestamp": T0, "kind": "internal"},
    ]}
    response = analyze(make_service(provider=MockProvider(scenario)), a, depth=1)
    assert len(response.transactions) == 1
    assert response.transactions[0].amount == "3.500000000000000000"
    amounts = sorted(e.amount for e in response.fund_flow)
    assert amounts == ["1.500000000000000000", "2.000000000000000000"]  # (a->b) merged, (a->c)


def test_cycle_back_to_suspect_uses_submitted_spelling(make_service):
    a, b = SUSPECT, "0x" + "b" * 40
    scenario = {"transfers": [
        {"tx_hash": "0x" + "01" * 32, "from": a, "to": b, "value_eth": "1", "timestamp": T0},
        {"tx_hash": "0x" + "02" * 32, "from": b, "to": a, "value_eth": "0.9", "timestamp": T0 + 10},
    ]}
    response = analyze(make_service(provider=MockProvider(scenario)), SUSPECT_CHECKSUM, depth=3)
    assert [x.address for x in response.addresses].count(SUSPECT_CHECKSUM) == 1
    assert len(response.addresses) == 2  # no duplicate row for the suspect
    assert response.fund_flow[1].destination_address == SUSPECT_CHECKSUM


# ----------------------------------------------------------------- attribution policy


def hit(name, status, depth, wei, addr):
    return VaspHit(VaspEntry(name, "exchange", status, addr), depth, wei)


def test_attribution_prefers_status_then_depth_then_amount():
    known_deep = hit("A", "known", 4, 1, "0x" + "1" * 40)
    probable_shallow = hit("B", "probable", 1, 9, "0x" + "2" * 40)
    assert choose_attribution([probable_shallow, known_deep]).entry.name == "A"

    near = hit("C", "known", 2, 1, "0x" + "3" * 40)
    assert choose_attribution([known_deep, near]).entry.name == "C"

    big = hit("D", "known", 2, 10, "0x" + "4" * 40)
    assert choose_attribution([near, big]).entry.name == "D"
    assert choose_attribution([]) is None


# --------------------------------------------------------------------- errors / budget


class SlowProvider(MockProvider):
    async def get_outgoing_transfers(self, address):
        await asyncio.sleep(1)
        return await super().get_outgoing_transfers(address)


def test_time_budget_exceeded_is_a_clean_analysis_failed(make_service, settings):
    provider = SlowProvider.from_file(settings.mock_scenario_file)
    with pytest.raises(AnalysisError) as err:
        analyze(make_service(provider=provider, budget_seconds=0.05), SUSPECT)
    assert err.value.code == CODE_ANALYSIS_FAILED and err.value.http_status == 500
    assert "time budget" in err.value.message


class BuggyProvider(MockProvider):
    async def get_outgoing_transfers(self, address):
        raise RuntimeError("secret internal detail")


def test_unexpected_exception_is_wrapped_without_leaking_details(make_service, settings):
    provider = BuggyProvider.from_file(settings.mock_scenario_file)
    with pytest.raises(AnalysisError) as err:
        analyze(make_service(provider=provider), SUSPECT)
    assert err.value.code == CODE_ANALYSIS_FAILED
    assert "secret" not in err.value.message


# ------------------------------------------------------------------------- validator


@pytest.fixture
def good(make_service) -> dict:
    return json.loads(analyze(make_service(), SUSPECT).model_dump_json())


def check(payload, wallet=SUSPECT, depth=5):
    validate_response(AnalysisResponse.model_validate(payload), wallet, depth)


def test_validator_accepts_a_good_response(good):
    check(good)


@pytest.mark.parametrize(
    "mutate, fragment",
    [
        (lambda p: p.update(wallet_address="0x" + "1" * 40), "echo"),
        (lambda p: p["addresses"].pop(0), "suspect"),
        (lambda p: p["addresses"].append(dict(p["addresses"][1])), "duplicate address"),
        (lambda p: p["fund_flow"][0].update(destination_address="0x" + "f" * 40), "not in addresses"),
        (lambda p: p["fund_flow"][0].update(amount="5"), "18-decimal"),
        (lambda p: p["transactions"][0].update(amount="5.0"), "18-decimal"),
        (lambda p: p["transactions"][0].update(tx_time="2026-09-16 18:30:00"), "ISO 8601"),
        (lambda p: p["transactions"][0].update(trace_depth=9), "trace_depth"),
        (lambda p: p["transactions"].pop(), "same tx_hash"),
        (lambda p: p["vasp_attribution"].update(matched_address=SUSPECT), "destination"),
    ],
)
def test_validator_rejects_contract_violations(good, mutate, fragment):
    mutate(good)
    with pytest.raises(AnalysisError) as err:
        check(good)
    assert err.value.code == CODE_ANALYSIS_FAILED
    assert fragment in err.value.message
