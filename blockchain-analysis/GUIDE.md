# Implementation Guide — Blockchain Analysis Service

This guide explains **what was built, why, and how each file works**, in the order it was built.
Read it top to bottom once, then use the file map (section 2) as a reference.

---

## 1. The big picture

### 1.1 What problem does this component solve?
A victim reports a wallet address they paid. The investigator needs to know **where the money went**.
This service takes that one address and returns a graph: which wallets received the money, in how many hops,
and whether it reached a **known exchange** (where a legal request can identify the account holder).

### 1.2 Where it sits in the system

```
Investigator → Go backend ──POST /analysis──▶ THIS SERVICE ──▶ blockchain data (Etherscan)
                  │                                   └──────▶ exchange list (JSON file)
                  ▼
              PostgreSQL
```

The Go backend owns the API, the database and the investigation lifecycle. This service is **stateless**:
one request in, one JSON graph out. It never touches the database (contract section 2).

### 1.3 The contract in one minute
Input `{wallet_address, max_depth}` → output `{wallet_address, addresses[], transactions[], fund_flow[], vasp_attribution|null}`.
Rules that shaped the code: amounts are **decimal strings**, times are **ISO 8601 UTC**, only on-chain identifiers
(no database IDs), errors use `{"error": {"code","message"}}`, and the whole call must finish within **30 seconds**.

---

## 2. File map

| File | One-line purpose |
|---|---|
| `app/config.py` | All settings from environment variables, validated at startup |
| `app/errors.py` | `AnalysisError` + the 4 contract error codes |
| `app/models.py` | Request/response models = the contract as code |
| `app/domain.py` | Internal data structures (`Transfer`, `Edge`, `TraceResult`…) |
| `app/formatting.py` | wei → decimal string, unix → ISO time, address normalising |
| `app/providers/base.py` | The `TransferProvider` interface (the seam to the blockchain) |
| `app/providers/mock.py` | Fake blockchain from `data/demo_scenario.json` |
| `app/providers/etherscan.py` | Real Ethereum data via Etherscan API V2 |
| `app/providers/ratelimit.py` | Spaces out API calls |
| `app/vasp.py` + `app/data/vasp_addresses.json` | Known exchange registry + loader |
| `app/tracer.py` | **The BFS algorithm and role assignment** |
| `app/builder.py` | TraceResult → contract JSON (aggregation, ordering, attribution choice) |
| `app/contract_validator.py` | Checks our own response against the contract |
| `app/service.py` | Orchestrates one request, owns the time budget |
| `app/main.py` | FastAPI app, error envelope handlers |
| `tests/` | 103 tests |

Data flow for one request:

```
HTTP body → AnalysisRequest (validate) → FundTracer.trace (BFS) → TraceResult
         → build_response (shape) → validate_response (self-check) → JSON
```

---

## 3. Step-by-step explanation

### Step 1 — Configuration (`config.py`)
Every tunable lives in one frozen `Settings` object read from environment variables (see `.env.example`).
* **Why:** no magic numbers in logic; bad values fail at **startup**, not mid-investigation.
* **`PROVIDER=mock|etherscan`** switches the data source without code changes.
* **ETH amounts use `Decimal`:** `"0.001"` → exactly `10**15` wei. Floats would introduce rounding errors.
* **`BUDGET_SECONDS=25`:** the Go client gives up at 30 s; we stop first and answer properly.

### Step 2 — Contract models and errors (`models.py`, `errors.py`, `domain.py`)
* `AnalysisRequest` validates the input: address is `0x` + 40 hex, `max_depth` is a *strict* integer 1–5.
  (EIP-55 checksum validation is the Go backend's job; we only check structure.)
* `AnalysisError` has constructors `invalid_request` (400), `analysis_failed` (500), `rpc_timeout` (502),
  `rate_limited` (502). Anywhere in the code you can `raise AnalysisError.rpc_timeout("…")`; `main.py` converts it to HTTP.
* `domain.py` holds **internal** objects. Contract models go *out*; domain objects are used *inside*.
  They are separate so the algorithm and the contract can evolve independently.

### Step 3 — Formatting (`formatting.py`)
* `wei_to_eth_string(1_500_000_000_000_000_000)` → `"1.500000000000000000"` using **integer** `divmod`, never floats.
* `unix_to_iso_utc(ts)` → `YYYY-MM-DDTHH:MM:SSZ`.
* `normalize_address` → lowercase, so `0xABC…` and `0xabc…` are the same wallet.

### Step 4 — Providers (`providers/`)
The tracer asks a `TransferProvider` two questions: *"what ETH did this address send?"* and *"is this a contract?"*.

**EtherscanProvider** calls three endpoints:
* `txlist` — normal transactions (a wallet sends ETH)
* `txlistinternal` — **internal** transfers (a smart contract forwards ETH). Laundering often uses forwarding
  contracts; without this endpoint the trail would break at the contract.
* `eth_getCode` — does the address hold contract code?

Row cleaning: failed/reverted transactions, incoming rows, zero-value rows and rows without a recipient are dropped;
for contract creations the new contract address is the recipient.

Error mapping: timeout / connection refused / HTTP 5xx → `RPC_TIMEOUT`; "rate limit" text or HTTP 429 (after retries) →
`RATE_LIMITED`; bad API key, non-JSON, unparseable row → `ANALYSIS_FAILED`.

**MockProvider** replays `app/data/demo_scenario.json`: a laundering chain with a cycle, dust, a failed transaction,
a contract, a pre-arrival transfer, plus addresses that simulate upstream errors. It is what the tests and demos use.

### Step 5 — VASP registry (`vasp.py`, `data/vasp_addresses.json`)
A lookup `address → (name, type, status)`. The chain never reveals ownership, so this is *external knowledge* kept in a
data file. It is validated on load (bad address, bad status, same address under two entities → refuses to start).
The shipped file is a **seed**: Binance 14, Binance 16, Coinbase 1. Re-verify and extend it before real casework.

### Step 6 — The tracer (`tracer.py`) — the core

**Algorithm: breadth-first search.** One loop iteration = one hop level.

```
frontier = [suspect]
for depth in 1..max_depth:
    1. fetch outgoing transfers of every wallet in frontier       (concurrently, throttled)
    2. keep eligible ones (not dust, not before arrival, top-N by value)  → record as edges at this depth
    3. every wallet seen for the first time becomes a node
       - known exchange  → recorded as a VASP hit, NOT expanded
       - otherwise       → next frontier (largest incoming amount first)
```

**Why these rules?**

| Rule | Reason |
|---|---|
| Global "seen" set | Cycles (funds returning) can't loop forever; each wallet is fetched once |
| Exchange not expanded | Contract halting rule 2; an exchange's own activity is irrelevant |
| **Temporal order** | Money can't leave a wallet before it arrived. A transfer made earlier is a *different* money flow; following it would draw a false path |
| Dust filter (0.001 ETH) | "Dusting" spam would flood the graph |
| Top-N per wallet (10) | A busy wallet could have thousands of outgoing transfers |
| `MAX_EXPANSIONS` (40) | Bounds API calls so the 25 s budget is realistic; biggest flows are explored first |

**Roles** (`_assign_roles`):

| Role | When |
|---|---|
| `suspect` | the reported wallet |
| `intermediary` | funds arrived **and** we saw funds leave |
| `destination` | a known exchange, or a wallet where the trace stopped (no outgoing found / depth limit reached) |
| `other` | a smart contract at the end of a branch (checked via `is_contract`, bounded by `MAX_CONTRACT_CHECKS`) |

#### Worked example (the demo data, `max_depth=5`)

```
SUSPECT ─5 ETH→ M1 ─4.9→ M2 ─4.8→ DEPOSIT ─4.79→ BINANCE_14 (known exchange)
   │             │          └─0.1→ M1                 (cycle: recorded, M1 NOT re-expanded)
   │             └─1→ TERM        (sent BEFORE funds reached M1 → ignored)
   ├─2→ M3 ─1.9→ TERM
   ├─0.5→ CONTRACT                (smart contract → role "other")
   ├─0.0001→ DUST                 (below threshold → ignored)
   └─1→ TERM                      (FAILED transaction → never happened)
```

Level by level: depth 1 finds M1, M3, CONTRACT. Depth 2 finds M2 and TERM. Depth 3 finds DEPOSIT (and the M2→M1 edge).
Depth 4 finds BINANCE_14, which is a known exchange → attribution. With `max_depth=3` the trace stops at DEPOSIT and
`vasp_attribution` is `null`.

### Step 6b — Builder and validator (`builder.py`, `contract_validator.py`)

**Builder decisions worth remembering**
1. **One transaction can contain several transfers** (a contract splitting funds). The DB keeps one row per `tx_hash`,
   so `transactions[]` has one item per hash (amounts summed, smallest depth) and `fund_flow[]` merges identical
   `(tx_hash, source, destination)` edges.
2. **The suspect keeps its submitted spelling**; all other addresses are lowercase. The backend stores addresses
   case-sensitively and saved the suspect exactly as typed. Returning a different spelling would create a duplicate wallet row.
3. **Only one `vasp_attribution`.** If several exchange wallets are reached we pick by: status (known > probable > unverified),
   then fewest hops, then largest amount received, then address.
4. **Deterministic ordering** → same input, byte-identical JSON.

**Validator** = contract section 9 checklist as code, run on every response: echo of wallet, suspect present once,
unique addresses, every edge endpoint in `addresses`, 18-decimal amount strings, ISO times, depth in range, matching
hash sets, attribution points to a `destination`. A violation means *our* bug, so it fails loudly with `ANALYSIS_FAILED`
rather than letting the backend persist a wrong graph.

### Step 6c — Service (`service.py`)
Runs trace → build → validate under an `asyncio.timeout(BUDGET_SECONDS)`.
**On timeout it fails** (it does not return a partial graph): the contract has no "incomplete" flag, so a partial result
would be stored as a normal completed investigation and could be mistaken for the full picture.
Unexpected exceptions are logged with a stack trace but returned as a generic message (no internals leaked).

### Step 7 — HTTP layer (`main.py`)
`POST /analysis`, `GET /health`. Overrides FastAPI's default 422 validation response so *every* error — bad JSON,
bad field, wrong method, unknown path — uses `{"error": {"code","message"}}`.

### Step 8 — Tests (`tests/`)
| File | Covers |
|---|---|
| `test_formatting.py` | exact wei conversion (incl. > float precision), ISO time, address normalising |
| `test_providers.py` | Etherscan row cleaning, retries, every error mapping (via `httpx.MockTransport`), mock provider |
| `test_vasp.py` | registry loading and validation |
| `test_tracer.py` | BFS levels, roles, dust/failed/pre-arrival filtering, cycles, caps, errors |
| `test_service.py` | contract shape, null attribution, aggregation, address spelling, attribution policy, time budget, validator rejections |
| `test_api.py` | HTTP behaviour: success, all 400 cases, 502/500 mappings, envelope on 404/405 |
| `test_config.py` | env parsing and startup validation |

Additionally the real Go client (`internal/orchestrator/client.go`) was compiled unchanged and run against the live
service: it decoded the response, the timestamps, the `null` attribution and every error code.

---

## 4. How to run it

```bash
cd blockchain-analysis
pip install -r requirements-dev.txt
make test                     # 103 tests
make run                      # mock blockchain on :9090, no API key needed
```

Connect your Go backend: in the repo root `.env`, `CFT_ANALYSIS_URL=http://localhost:9090`, then `make run` for Go and:

```bash
curl -X POST http://localhost:8080/api/v1/investigations -H "Content-Type: application/json" \
  -d '{"wallet_address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}'
```
(With the mock provider, any address not in the demo data simply returns a graph containing only the suspect.
Demo suspects: `0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed` for the laundering chain, `0x742d35cc…f44e` for the direct-to-Binance case.
Addresses ending in `…b1`/`…b2`/`…b3` (40 × `b` then 1/2/3) simulate `RPC_TIMEOUT`, `RATE_LIMITED`, `ANALYSIS_FAILED`.)

### Going live with real data
1. Get a free API key from Etherscan.
2. `cp .env.example .env`, set `PROVIDER=etherscan` and `ETHERSCAN_API_KEY=…`.
3. **Smoke test first:** `make smoke-live` (one address, depth 1, a handful of API calls). This is the one part that could
   not be tested against the real API during development (no internet in the build environment).
4. `make run-live`.

---

## 5. Known limitations (be upfront about these in your report)

1. **Native ETH only.** USDT/USDC (ERC-20 tokens) are extremely common in scam cases and are **not traced**. The contract has
   no `asset` field; supporting tokens needs a contract extension (token symbol + decimals) and Etherscan's `tokentx` endpoint.
2. **Most recent 1,000 transactions per address** (per endpoint). Very busy wallets are only partly covered.
3. **One chain** (Ethereum mainnet). No bridges, mixers or cross-chain tracing (MVP scope).
4. **Tiny exchange registry.** Exchanges usually receive funds through per-customer *deposit addresses* that sweep to a hot
   wallet. The trace catches this only if the sweep happens within `max_depth` and the hot wallet is in the registry.
5. **Timeouts fail instead of returning partial results** (a deliberate evidence-safety choice).
6. **Heuristic caps** (top-10 per wallet, 40 wallets, dust threshold) can drop minor branches; truncation is logged.
7. **Etherscan provider not yet exercised against the live API** — run `make smoke-live` with your key.

## 6. Integration notes for the Go backend (outside this service, worth fixing)

* `transaction_edge` has no `investigation_id` and no unique constraint. If two investigations contain the same transaction,
  its edges are inserted twice and then appear twice in each result. Add an `investigation_id` (or a unique key) to the edge table.
* Addresses are case-sensitive in `address.address`. Normalising to lowercase in the Go validator/repository would make the
  system robust regardless of what any analysis component returns.
* `GetInvestigation` only finds an attribution if the matched address has role `destination` (this service guarantees that).

## 7. Extending the service

* **New data source:** implement `TransferProvider` (two methods), register it in `providers/__init__.py`.
* **More exchanges:** add entities/addresses to `app/data/vasp_addresses.json`; the loader validates them.
* **Token tracing:** add `asset` to `Transfer`, a `tokentx` fetch in the provider, and an `asset` field to the contract.
* **Caching:** cache provider results per address (in-memory/Redis) to save API quota; the service stays stateless in behaviour.
