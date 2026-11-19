"""Tests of the BFS algorithm against the demo blockchain (app/data/demo_scenario.json).

The demo graph (amounts in ETH):

    SUSPECT -5-> M1 -4.9-> M2 -4.8-> DEP -4.79-> BINANCE_14
       |          |          '-0.1-> M1   (a cycle)
       |          '-1-> TERM  (sent BEFORE funds reached M1: must be ignored)
       |-2-> M3 -1.9-> TERM
       |-0.5-> CONTRACT (a smart contract)
       |-0.0001-> DUST  (below the dust threshold)
       '-1-> TERM (FAILED transaction: never happened)
"""

import pytest

from app.errors import CODE_RATE_LIMITED, CODE_RPC_TIMEOUT, AnalysisError
from tests.conftest import (
    BINANCE_14, CONTRACT, DEP, DUST, ERR_RATE, ERR_TIMEOUT, M1, M2, M3, SIMPLE_SUSPECT, SUSPECT, TERM, run,
)


def edges_of(result):
    return {(e.source, e.destination, e.depth) for e in result.edges}


def test_full_trace_reaches_binance_at_depth_4(make_tracer):
    tracer, _ = make_tracer()
    result = run(tracer.trace(SUSPECT, 5))

    assert result.depth_of == {SUSPECT: 0, M1: 1, M3: 1, CONTRACT: 1, M2: 2, TERM: 2, DEP: 3, BINANCE_14: 4}
    assert edges_of(result) == {
        (SUSPECT, M1, 1), (SUSPECT, M3, 1), (SUSPECT, CONTRACT, 1),
        (M1, M2, 2), (M3, TERM, 2),
        (M2, DEP, 3), (M2, M1, 3),
        (DEP, BINANCE_14, 4),
    }
    assert [h.entry.name for h in result.vasp_hits] == ["Binance"]
    assert result.vasp_hits[0].depth == 4


def test_roles(make_tracer):
    tracer, _ = make_tracer()
    roles = run(tracer.trace(SUSPECT, 5)).roles
    assert roles == {
        SUSPECT: "suspect",
        M1: "intermediary", M2: "intermediary", M3: "intermediary", DEP: "intermediary",
        TERM: "destination",       # trace ended here: no outgoing transfers
        BINANCE_14: "destination",  # known exchange
        CONTRACT: "other",          # smart contract at the end of a branch
    }


def test_dust_failed_and_pre_arrival_transfers_are_ignored(make_tracer):
    tracer, _ = make_tracer()
    result = run(tracer.trace(SUSPECT, 5))
    assert DUST not in result.depth_of  # dust filtered
    assert (M1, TERM, 2) not in edges_of(result)  # sent before the funds arrived
    assert (SUSPECT, TERM, 1) not in edges_of(result)  # failed tx never reaches the tracer


def test_disabling_temporal_order_lets_pre_arrival_transfer_through(make_tracer):
    tracer, _ = make_tracer(enforce_temporal_order=False)
    result = run(tracer.trace(SUSPECT, 5))
    assert (M1, TERM, 2) in edges_of(result)


def test_depth_limit_stops_before_binance(make_tracer):
    tracer, _ = make_tracer()
    result = run(tracer.trace(SUSPECT, 3))
    assert BINANCE_14 not in result.depth_of
    assert result.vasp_hits == []
    assert max(result.depth_of.values()) == 3
    assert result.roles[DEP] == "destination"  # frontier ended here because of max_depth


def test_depth_one_only_direct_transfers(make_tracer):
    tracer, _ = make_tracer()
    result = run(tracer.trace(SUSPECT, 1))
    assert edges_of(result) == {(SUSPECT, M1, 1), (SUSPECT, M3, 1), (SUSPECT, CONTRACT, 1)}


def test_cycle_does_not_loop_and_wallet_is_fetched_once(make_tracer):
    tracer, provider = make_tracer()
    run(tracer.trace(SUSPECT, 5))
    assert provider.calls.count(M1) == 1  # M2 -> M1 closes a cycle; M1 is not expanded again
    assert len(provider.calls) == len(set(provider.calls))


def test_exchange_is_never_expanded(make_tracer):
    tracer, provider = make_tracer()
    run(tracer.trace(SUSPECT, 5))
    assert BINANCE_14 not in provider.calls


def test_direct_to_exchange_case(make_tracer):
    tracer, _ = make_tracer()
    result = run(tracer.trace(SIMPLE_SUSPECT, 3))
    assert result.depth_of == {SIMPLE_SUSPECT: 0, BINANCE_14: 1}
    assert result.roles == {SIMPLE_SUSPECT: "suspect", BINANCE_14: "destination"}
    assert result.vasp_hits[0].depth == 1


def test_wallet_with_no_activity_returns_only_the_suspect(make_tracer):
    tracer, _ = make_tracer()
    result = run(tracer.trace("0x" + "9" * 40, 5))
    assert result.depth_of == {"0x" + "9" * 40: 0}
    assert result.edges == [] and result.vasp_hits == []


def test_suspect_address_case_does_not_matter(make_tracer):
    tracer, _ = make_tracer()
    result = run(tracer.trace("0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed", 1))
    assert result.suspect == SUSPECT and len(result.edges) == 3


def test_fanout_cap_keeps_only_the_largest_transfers(make_tracer):
    tracer, _ = make_tracer(max_transfers_per_address=1)
    result = run(tracer.trace(SUSPECT, 1))
    assert edges_of(result) == {(SUSPECT, M1, 1)}  # 5 ETH is the largest
    assert result.truncated_addresses == 1


def test_expansion_cap_limits_lookups_and_prefers_biggest_flows(make_tracer):
    tracer, provider = make_tracer(max_expansions=2)
    result = run(tracer.trace(SUSPECT, 5))
    assert result.hit_expansion_cap is True
    assert provider.calls == [SUSPECT, M1]  # biggest incoming (5 ETH) first; then the cap is reached
    assert result.expansions == 2


def test_contract_checks_can_be_disabled(make_tracer):
    tracer, _ = make_tracer(check_contracts=False)
    assert run(tracer.trace(SUSPECT, 5)).roles[CONTRACT] == "destination"


def test_upstream_errors_propagate_with_their_codes(make_tracer):
    tracer, _ = make_tracer()
    with pytest.raises(AnalysisError) as err:
        run(tracer.trace(ERR_TIMEOUT, 3))
    assert err.value.code == CODE_RPC_TIMEOUT
    with pytest.raises(AnalysisError) as err:
        run(tracer.trace(ERR_RATE, 3))
    assert err.value.code == CODE_RATE_LIMITED
