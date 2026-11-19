import pytest

from app.config import Settings
from app.vasp import VaspRegistry

BINANCE_14 = "0x28c6c06298d514db089934071355e5743bf21d60"


def test_shipped_registry_loads_and_looks_up_case_insensitively():
    registry = VaspRegistry.from_file(Settings.for_testing().vasp_file)
    assert len(registry) >= 3
    entry = registry.lookup("0x28C6c06298d514Db089934071355E5743bf21d60")  # checksum-case
    assert entry is not None
    assert (entry.name, entry.type, entry.attribution_status) == ("Binance", "exchange", "known")
    assert entry.address == BINANCE_14


def test_unknown_address_returns_none():
    registry = VaspRegistry.from_file(Settings.for_testing().vasp_file)
    assert registry.lookup("0x" + "1" * 40) is None


def _entity(name="X", status="known", addresses=(BINANCE_14,)):
    return {"name": name, "type": "exchange", "attribution_status": status,
            "addresses": [{"address": a} for a in addresses]}


def test_rejects_bad_address():
    with pytest.raises(ValueError, match="invalid address"):
        VaspRegistry.from_dict({"entities": [_entity(addresses=("0x1234",))]})


def test_rejects_bad_status():
    with pytest.raises(ValueError, match="attribution_status"):
        VaspRegistry.from_dict({"entities": [_entity(status="sure")]})


def test_rejects_address_listed_under_two_entities():
    with pytest.raises(ValueError, match="appears twice"):
        VaspRegistry.from_dict({"entities": [_entity("A"), _entity("B")]})
