import pytest

from app.config import Settings, load_env_file


def test_defaults_from_empty_environment(monkeypatch):
    for name in ("PROVIDER", "BUDGET_SECONDS", "MIN_VALUE_ETH", "ETHERSCAN_API_KEY", "PORT"):
        monkeypatch.delenv(name, raising=False)
    s = Settings.from_env()
    assert s.provider == "mock" and s.port == 9090 and s.budget_seconds == 25.0
    assert s.min_value_wei == 10**15  # "0.001" ETH, converted exactly


def test_etherscan_requires_api_key(monkeypatch):
    monkeypatch.setenv("PROVIDER", "etherscan")
    monkeypatch.delenv("ETHERSCAN_API_KEY", raising=False)
    with pytest.raises(ValueError, match="ETHERSCAN_API_KEY"):
        Settings.from_env()


@pytest.mark.parametrize(
    "name, value",
    [("PROVIDER", "bitcoin"), ("BUDGET_SECONDS", "0"), ("CONCURRENCY", "0"),
     ("MAX_EXPANSIONS", "x"), ("MIN_VALUE_ETH", "abc"), ("MAX_TRANSFERS_PER_ADDRESS", "0")],
)
def test_bad_values_fail_at_startup(monkeypatch, name, value):
    monkeypatch.setenv(name, value)
    with pytest.raises(ValueError):
        Settings.from_env()


def test_env_file_does_not_override_real_environment(monkeypatch, tmp_path):
    f = tmp_path / ".env"
    f.write_text("# comment\nPORT=1234\nHOST='127.0.0.1'\n\nBROKEN LINE\n")
    monkeypatch.setenv("PORT", "9999")
    monkeypatch.delenv("HOST", raising=False)
    load_env_file(f)
    import os
    assert os.environ["PORT"] == "9999" and os.environ["HOST"] == "127.0.0.1"
    monkeypatch.delenv("HOST")
