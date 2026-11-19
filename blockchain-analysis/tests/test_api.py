"""HTTP-level tests: what the Go backend actually sees on the wire."""

import pytest
from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app
from tests.conftest import (
    BINANCE_14, ERR_FAILED, ERR_RATE, ERR_TIMEOUT, SIMPLE_SUSPECT, SUSPECT, SUSPECT_CHECKSUM,
)

from app.providers.mock import MockProvider


@pytest.fixture(scope="module")
def client():
    settings = Settings.for_testing()
    provider = MockProvider.from_file(settings.mock_scenario_file)
    with TestClient(create_app(settings, provider)) as c:  # `with` runs the startup hook
        yield c


def post(client, body):
    return client.post("/analysis", json=body)


def assert_envelope(response, status, code):
    assert response.status_code == status, response.text
    assert response.headers["content-type"].startswith("application/json")
    body = response.json()
    assert set(body) == {"error"}
    assert set(body["error"]) == {"code", "message"}
    assert body["error"]["code"] == code
    assert body["error"]["message"]


def test_success_returns_contract_payload(client):
    response = post(client, {"wallet_address": SUSPECT_CHECKSUM, "max_depth": 5})
    assert response.status_code == 200
    assert response.headers["content-type"].startswith("application/json")
    body = response.json()
    assert set(body) == {"wallet_address", "addresses", "transactions", "fund_flow", "vasp_attribution"}
    assert body["wallet_address"] == SUSPECT_CHECKSUM
    assert body["vasp_attribution"]["matched_address"] == BINANCE_14
    assert set(body["transactions"][0]) == {"tx_hash", "tx_time", "amount", "trace_depth", "raw_reference"}
    assert set(body["fund_flow"][0]) == {"tx_hash", "source_address", "destination_address", "amount"}
    assert isinstance(body["transactions"][0]["amount"], str)  # decimal STRING, never a number


def test_max_depth_is_optional_and_defaults_to_3(client):
    body = post(client, {"wallet_address": SUSPECT}).json()
    assert body["vasp_attribution"] is None  # Binance is 4 hops away; default depth is 3
    assert max(t["trace_depth"] for t in body["transactions"]) == 3


def test_readme_example_request(client):
    body = post(client, {"wallet_address": SIMPLE_SUSPECT, "max_depth": 3}).json()
    assert body["vasp_attribution"]["name"] == "Binance"


def test_unknown_extra_fields_are_ignored(client):
    assert post(client, {"wallet_address": SUSPECT, "max_depth": 1, "future_field": 1}).status_code == 200


@pytest.mark.parametrize(
    "body",
    [
        {},
        {"max_depth": 3},
        {"wallet_address": ""},
        {"wallet_address": "0x1234"},
        {"wallet_address": "0x" + "g" * 40},
        {"wallet_address": "5aaeb6053f3e94c9b9a09f33669435e7ef1beaed"},  # missing 0x
        {"wallet_address": SUSPECT, "max_depth": 0},
        {"wallet_address": SUSPECT, "max_depth": 6},
        {"wallet_address": SUSPECT, "max_depth": "3"},
        {"wallet_address": SUSPECT, "max_depth": 2.5},
        {"wallet_address": 12345},
    ],
)
def test_bad_requests_get_400_invalid_request(client, body):
    assert_envelope(post(client, body), 400, "INVALID_REQUEST")


def test_malformed_json_is_400(client):
    response = client.post("/analysis", content="{not json", headers={"Content-Type": "application/json"})
    assert_envelope(response, 400, "INVALID_REQUEST")


def test_empty_body_is_400(client):
    assert_envelope(client.post("/analysis"), 400, "INVALID_REQUEST")


def test_invalid_request_message_names_the_field(client):
    message = post(client, {"wallet_address": SUSPECT, "max_depth": 9}).json()["error"]["message"]
    assert "max_depth" in message


@pytest.mark.parametrize(
    "wallet, status, code",
    [(ERR_TIMEOUT, 502, "RPC_TIMEOUT"), (ERR_RATE, 502, "RATE_LIMITED"), (ERR_FAILED, 500, "ANALYSIS_FAILED")],
)
def test_upstream_failures_map_to_contract_codes(client, wallet, status, code):
    assert_envelope(post(client, {"wallet_address": wallet, "max_depth": 2}), status, code)


def test_wrong_method_and_path_use_the_same_envelope(client):
    assert client.get("/analysis").status_code == 405
    assert "error" in client.get("/analysis").json()
    assert client.get("/nope").status_code == 404


def test_health(client):
    assert client.get("/health").json() == {"status": "ok", "provider": "mock"}
