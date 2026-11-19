# Blockchain Analysis Service

Python implementation of the **Blockchain Analysis** component of Crypto Fund Tracer. It serves `POST /analysis` exactly as
specified in [`docs/lld/blockchain-analysis-integration-contract.md`](../docs/lld/blockchain-analysis-integration-contract.md):
given a suspect Ethereum address and a depth, it traces outgoing ETH transfers hop by hop, assigns address roles, and
reports whether the funds reached a known exchange/VASP.

**Read [`GUIDE.md`](GUIDE.md) for the full step-by-step explanation.**

## Quick start

```bash
pip install -r requirements-dev.txt
make test        # 103 tests
make run         # serves on :9090 using the built-in mock blockchain (no API key)
```

```bash
curl -X POST localhost:9090/analysis -H 'Content-Type: application/json' \
  -d '{"wallet_address":"0x742d35cc6634c0532925a3b844bc454e4438f44e","max_depth":3}'
```

Real data: set `PROVIDER=etherscan` and `ETHERSCAN_API_KEY` in `.env` (see `.env.example`), run `make smoke-live`, then `make run-live`.

## Layout

```
app/            service code (config, models, providers, tracer, builder, validator, API)
app/data/       vasp_addresses.json (exchange registry), demo_scenario.json (mock blockchain)
tests/          pytest suite
scripts/        smoke_live.py (one real lookup)
```
