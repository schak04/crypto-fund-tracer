"""Tests for the providers. No real network: Etherscan is faked with httpx.MockTransport."""

import asyncio

import httpx
import pytest

from app.config import Settings
from app.errors import CODE_ANALYSIS_FAILED, CODE_RATE_LIMITED, CODE_RPC_TIMEOUT, AnalysisError
from app.providers.etherscan import EtherscanProvider
from app.providers.mock import MockProvider

ME = "0x1111111111111111111111111111111111111101"
OTHER = "0x2222222222222222222222222222222222222202"


def run(coro):
    return asyncio.run(coro)


def make_provider(handler, **settings_overrides) -> EtherscanProvider:
    settings = Settings.for_testing(provider="etherscan", **settings_overrides)
    client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
    return EtherscanProvider(settings, client)


def row(**kw):
    base = {
        "hash": "0x" + "ab" * 32,
        "from": ME,
        "to": OTHER,
        "value": "1500000000000000000",
        "timeStamp": "1789583400",
        "isError": "0",
        "txreceipt_status": "1",
    }
    base.update(kw)
    return base


def ok(rows):
    return httpx.Response(200, json={"status": "1", "message": "OK", "result": rows})


# ----------------------------------------------------------------- parsing rules


def test_keeps_valid_outgoing_transfer_and_normalises_case():
    def handler(request):
        rows = [row(**{"from": ME.upper().replace("0X", "0x"), "to": OTHER.upper().replace("0X", "0x")})]
        return ok(rows if request.url.params["action"] == "txlist" else [])

    transfers = run(make_provider(handler).get_outgoing_transfers(ME))
    assert len(transfers) == 1
    t = transfers[0]
    assert (t.from_address, t.to_address) == (ME, OTHER)
    assert t.value_wei == 1_500_000_000_000_000_000
    assert t.timestamp == 1789583400
    assert t.kind == "normal"


def test_drops_incoming_failed_and_zero_value_rows():
    rows = [
        row(**{"from": OTHER, "to": ME}),  # incoming
        row(isError="1"),  # failed
        row(txreceipt_status="0"),  # reverted
        row(value="0"),  # plain contract call
        row(hash="0x" + "cd" * 32),  # the only good one
    ]

    def handler(request):
        return ok(rows if request.url.params["action"] == "txlist" else [])

    transfers = run(make_provider(handler).get_outgoing_transfers(ME))
    assert [t.tx_hash for t in transfers] == ["0x" + "cd" * 32]


def test_contract_creation_uses_contract_address_as_recipient():
    new_contract = "0x3333333333333333333333333333333333333303"

    def handler(request):
        rows = [row(to="", contractAddress=new_contract)]
        return ok(rows if request.url.params["action"] == "txlist" else [])

    transfers = run(make_provider(handler).get_outgoing_transfers(ME))
    assert transfers[0].to_address == new_contract


def test_internal_transfers_are_marked_internal():
    def handler(request):
        if request.url.params["action"] == "txlistinternal":
            return ok([row()])
        return ok([])

    transfers = run(make_provider(handler).get_outgoing_transfers(ME))
    assert [t.kind for t in transfers] == ["internal"]


def test_malformed_row_is_an_analysis_failure():
    def handler(request):
        return ok([{"hash": "0x1"}] if request.url.params["action"] == "txlist" else [])

    with pytest.raises(AnalysisError) as err:
        run(make_provider(handler).get_outgoing_transfers(ME))
    assert err.value.code == CODE_ANALYSIS_FAILED


def test_no_transactions_found_is_not_an_error():
    def handler(request):
        return httpx.Response(200, json={"status": "0", "message": "No transactions found", "result": []})

    assert run(make_provider(handler).get_outgoing_transfers(ME)) == []


def test_api_key_and_chain_are_sent():
    seen = {}

    def handler(request):
        seen.update(dict(request.url.params))
        return ok([])

    run(make_provider(handler).get_outgoing_transfers(ME))
    assert seen["apikey"] == "test-key"
    assert seen["chainid"] == "1"
    assert seen["address"] == ME


# ------------------------------------------------------------------ error mapping


def test_rate_limit_then_success_is_retried():
    calls = {"n": 0}

    def handler(request):
        calls["n"] += 1
        if calls["n"] == 1:
            return httpx.Response(200, json={"status": "0", "message": "NOTOK", "result": "Max rate limit reached"})
        return ok([])

    assert run(make_provider(handler, max_retries=2).get_outgoing_transfers(ME)) == []
    assert calls["n"] >= 3  # 1 failed + retry of txlist + txlistinternal


def test_persistent_rate_limit_becomes_rate_limited_error():
    def handler(request):
        return httpx.Response(200, json={"status": "0", "message": "NOTOK", "result": "Max rate limit reached"})

    with pytest.raises(AnalysisError) as err:
        run(make_provider(handler, max_retries=1).get_outgoing_transfers(ME))
    assert (err.value.code, err.value.http_status) == (CODE_RATE_LIMITED, 502)


def test_http_429_becomes_rate_limited_error():
    with pytest.raises(AnalysisError) as err:
        run(make_provider(lambda r: httpx.Response(429), max_retries=1).get_outgoing_transfers(ME))
    assert err.value.code == CODE_RATE_LIMITED


def test_timeout_becomes_rpc_timeout():
    def handler(request):
        raise httpx.ReadTimeout("slow", request=request)

    with pytest.raises(AnalysisError) as err:
        run(make_provider(handler).get_outgoing_transfers(ME))
    assert (err.value.code, err.value.http_status) == (CODE_RPC_TIMEOUT, 502)


def test_connection_failure_becomes_rpc_timeout():
    def handler(request):
        raise httpx.ConnectError("refused", request=request)

    with pytest.raises(AnalysisError) as err:
        run(make_provider(handler).get_outgoing_transfers(ME))
    assert err.value.code == CODE_RPC_TIMEOUT


def test_upstream_5xx_becomes_rpc_timeout():
    with pytest.raises(AnalysisError) as err:
        run(make_provider(lambda r: httpx.Response(503)).get_outgoing_transfers(ME))
    assert err.value.code == CODE_RPC_TIMEOUT


def test_invalid_api_key_becomes_analysis_failed():
    def handler(request):
        return httpx.Response(200, json={"status": "0", "message": "NOTOK", "result": "Invalid API Key"})

    with pytest.raises(AnalysisError) as err:
        run(make_provider(handler).get_outgoing_transfers(ME))
    assert err.value.code == CODE_ANALYSIS_FAILED
    assert "API key" in err.value.message


def test_non_json_body_becomes_analysis_failed():
    with pytest.raises(AnalysisError) as err:
        run(make_provider(lambda r: httpx.Response(200, text="<html>oops</html>")).get_outgoing_transfers(ME))
    assert err.value.code == CODE_ANALYSIS_FAILED


# --------------------------------------------------------------------- is_contract


@pytest.mark.parametrize(
    "code, expected",
    [("0x", False), ("0x6080604052", True), ("0xef0100" + "11" * 20, False)],
)
def test_is_contract_reads_proxy_style_response(code, expected):
    def handler(request):
        assert request.url.params["action"] == "eth_getCode"
        return httpx.Response(200, json={"jsonrpc": "2.0", "id": 1, "result": code})

    assert run(make_provider(handler).is_contract(ME)) is expected


# ------------------------------------------------------------------ mock provider


def test_mock_provider_demo_scenario_loads_and_filters_failed():
    settings = Settings.for_testing()
    provider = MockProvider.from_file(settings.mock_scenario_file)
    suspect = "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"
    transfers = run(provider.get_outgoing_transfers(suspect.upper().replace("0X", "0x")))
    # A1..A4 are sent by the suspect; A5 is FAILED and must not appear.
    assert len(transfers) == 4
    assert all(t.from_address == suspect for t in transfers)


def test_mock_provider_simulated_errors():
    provider = MockProvider.from_file(Settings.for_testing().mock_scenario_file)
    with pytest.raises(AnalysisError) as err:
        run(provider.get_outgoing_transfers("0x" + "b" * 39 + "1"))
    assert err.value.code == CODE_RPC_TIMEOUT
