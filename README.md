# Crypto Fund Tracer

A blockchain analytics system for tracing cryptocurrency funds from suspect wallet addresses and identifying their destination entities, particularly cryptocurrency exchanges and Virtual Asset Service Providers (VASPs).

## Overview

In cyber fraud and financial crime cases, victims are frequently instructed to transfer cryptocurrency to wallet addresses controlled by perpetrators. Tracing these funds manually across multiple intermediary wallets, hops, and transaction layers is slow and labour-intensive.

Crypto Fund Tracer automates this process by:

- Validating suspect wallet addresses and formatting rules.
- Ingesting and normalizing relevant on-chain transaction records.
- Traversing fund flows across multi-hop transactions.
- Reconstructing directed transaction graphs for investigative review.
- Matching destination addresses against known exchange and VASP registries.
- Persisting structured investigation records for auditing and querying.

## System Architecture

The application is split across two primary layers:

1. Application + Data-Access
2. Blockchain Analysis

![architecture](docs/diagrams/architecture.png)

## Team Responsibilities

### Application and Data-Access Layer (Go Backend) - [Saptaparno Chakraborty](https://github.com/schak04)

- **System Architecture and Specifications**: Designing the overall system architecture, technical specifications, data models, and the subsystem integration contract.
- **REST API and HTTP Transport**: Implementing the Go web server, route handling, request decoding, response serialisation, and graceful process shutdown.
- **Public Investigation Endpoints**: Exposing and serving the submission (`POST /api/v1/investigations`) and query (`GET /api/v1/investigations/{id}`) endpoints.
- **Address Validation and Checksums**: Implementing Ethereum address parsing, format verification, and EIP-55 Keccak-256 checksum validation.
- **Investigation Lifecycle and Orchestration**: Coordinating investigation state transitions (`pending`, `running`, `completed`, `failed`), asynchronous dispatch, and analysis service communication.
- **Relational Schema and Migrations**: Designing the normalized PostgreSQL database schema and managing database revisions via Goose SQL migrations.
- **Transactional Persistence and Graph Queries**: Implementing repository operations with `pgx`, connection pooling, and SQL joins to reconstruct multi-hop fund-flow graphs.
- **Testing and Development Tooling**: Developing unit and integration test suites, environment configuration loading, and Makefile automation.

### Blockchain Analysis Layer - [Sakshi](https://github.com/sakshik2023)

- **Service Interface and HTTP Transport**: Implementing and serving the `POST /analysis` HTTP endpoint conforming to the subsystem integration contract.
- **On-Chain Data Acquisition**: Connecting to blockchain RPC nodes and explorer APIs to fetch raw transaction receipts and logs for suspect addresses.
- **Transaction Parsing and Normalisation**: Parsing raw blockchain transaction formats and normalising fields, values, and timestamps into uniform data models.
- **Fund-Flow Graph Traversal**: Implementing graph traversal algorithms to trace outgoing fund transfers iteratively up to the configured recursion depth.
- **Address Role Categorisation**: Categorising wallet addresses discovered during traversal into contextual roles (`suspect`, `intermediary`, `destination`, `other`).
- **VASP and Exchange Attribution**: Matching identified destination addresses against known exchange and VASP registry datasets to attribute destination entities.
- **Response Serialisation and Validation**: Formatting and validating the structured analysis payload, ensuring decimal precision rules and integration contract compliance before returning data to the Go backend.

## Integration Contract

The integration boundary between the Go backend and the Blockchain Analysis component is strictly defined in [`docs/lld/blockchain-analysis-integration-contract.md`](docs/lld/blockchain-analysis-integration-contract.md).

Key architectural constraints for the Blockchain Analysis service:

- **Stateless execution**: Each `POST /analysis` request is self-contained. The analysis service does not maintain state across requests.
- **No direct database access**: The analysis service does not connect to PostgreSQL. All persistence is handled by the Go backend.
- **On-chain identifiers only**: Identifiers must be blockchain addresses and transaction hashes. Database UUID generation is managed by the Go backend.
- **Decimal string precision**: Monetary amounts must be returned as numeric decimal strings (never floating-point numbers) to prevent precision loss.
- **Timestamps**: All transaction confirmation timestamps must follow ISO 8601 UTC (`YYYY-MM-DDTHH:MM:SSZ`).
- **Execution timeout**: Analysis requests must complete within 30 seconds for MVP execution.

> [!IMPORTANT]  
> The wallet addresses and transaction hashes shown in the schemas and examples below are synthetic sample fixtures and public test vectors (including canonical EIP-55 fixtures and public exchange addresses). They are used solely for format illustration and local integration testing, and do not represent active investigations or private personal data.

### Request Schema (`POST /analysis`)

```json
{
  "wallet_address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
  "max_depth": 3
}
```

### Response Schema (`HTTP 200 OK`)

```json
{
  "wallet_address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
  "addresses": [
    {
      "address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
      "role": "suspect"
    },
    {
      "address": "0x28c6c06298d514db089934071355e5743bf21d60",
      "role": "destination"
    }
  ],
  "transactions": [
    {
      "tx_hash": "0x9f5a02bc73a1197775a2d6cc560ab9d96ff6d46d03d120a1db4a3b04c8695027",
      "tx_time": "2026-09-16T18:30:00Z",
      "amount": "1.500000000000000000",
      "trace_depth": 1,
      "raw_reference": "https://etherscan.io/tx/0x9f5a02bc73a1197775a2d6cc560ab9d96ff6d46d03d120a1db4a3b04c8695027"
    }
  ],
  "fund_flow": [
    {
      "tx_hash": "0x9f5a02bc73a1197775a2d6cc560ab9d96ff6d46d03d120a1db4a3b04c8695027",
      "source_address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
      "destination_address": "0x28c6c06298d514db089934071355e5743bf21d60",
      "amount": "1.500000000000000000"
    }
  ],
  "vasp_attribution": {
    "name": "Binance",
    "type": "exchange",
    "matched_address": "0x28c6c06298d514db089934071355e5743bf21d60",
    "attribution_status": "known"
  }
}
```

If no known exchange or VASP address is reached, `vasp_attribution` must be `null`.

For failure modes and error response envelopes, consult [`docs/lld/blockchain-analysis-integration-contract.md`](docs/lld/blockchain-analysis-integration-contract.md).

## Local Development and Integration

To test the full system locally across both components:

1. Configure environment variables in `.env` (derived from `.env.example`):
   - `CFT_ANALYSIS_URL`: Address of the blockchain analysis service (default: `http://localhost:9090`).
   - `CFT_DB_URL`: PostgreSQL connection string.
2. Run database migrations:
   ```bash
   make migrate-up
   ```
3. Start the Go backend:
   ```bash
   make run
   ```
4. Start the blockchain analysis service on the port specified by `CFT_ANALYSIS_URL`.
5. Trigger an end-to-end investigation:
   ```bash
   curl -X POST http://localhost:8080/api/v1/investigations \
     -H "Content-Type: application/json" \
     -d '{"wallet_address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e"}'
   ```

## Configuration

| Variable                 | Default Value                                           | Description                                               |
| ------------------------ | ------------------------------------------------------- | --------------------------------------------------------- |
| `CFT_ENV`                | `development`                                           | Runtime environment (`development`, `test`, `production`) |
| `CFT_HTTP_ADDR`          | `:8080`                                                 | HTTP listen address for the REST API                      |
| `CFT_DB_URL`             | `postgres://cft:cft@localhost:5432/cft?sslmode=disable` | PostgreSQL connection string                              |
| `CFT_ANALYSIS_URL`       | `http://localhost:9090`                                 | Base URL of the blockchain analysis service               |
| `CFT_ANALYSIS_MAX_DEPTH` | `3`                                                     | Default fund-tracing depth limit (between 1 and 5)        |

## Makefile Commands

| Target                            | Description                                        |
| --------------------------------- | -------------------------------------------------- |
| `make run`                        | Starts the Go REST API server                      |
| `make test`                       | Executes all unit and integration tests            |
| `make db-ping`                    | Checks PostgreSQL connectivity using `psql`        |
| `make db-shell`                   | Launches an interactive `psql` session             |
| `make db-status`                  | Displays PostgreSQL service status via `systemctl` |
| `make migrate-status`             | Inspects database migration status                 |
| `make migrate-up`                 | Applies all pending migrations                     |
| `make migrate-down`               | Rolls back the most recent migration               |
| `make migrate-create NAME=<name>` | Generates a new SQL migration file                 |

## Documentation Index

- **Problem Context**: [`docs/problem.md`](docs/problem.md)
- **MVP Scope and Boundaries**: [`docs/mvp.md`](docs/mvp.md)
- **High-Level Architecture**: [`docs/hld/architecture.md`](docs/hld/architecture.md)
- **Blockchain Analysis Integration Contract**: [`docs/lld/blockchain-analysis-integration-contract.md`](docs/lld/blockchain-analysis-integration-contract.md)
- **REST API Specification**: [`docs/lld/api.md`](docs/lld/api.md)
- **Database Schema and Entities**: [`docs/lld/database.md`](docs/lld/database.md)
- **Configuration Design**: [`docs/lld/config.md`](docs/lld/config.md)
- **Database Connection and Tooling**: [`docs/lld/database-connection-and-tooling.md`](docs/lld/database-connection-and-tooling.md)

## References

- [pgx: PostgreSQL Driver and Toolkit for Go](https://pkg.go.dev/github.com/jackc/pgx/v5)
- [goose: Database Migration Tool](https://pkg.go.dev/github.com/pressly/goose/v3)
- [Go Context Package Documentation](https://pkg.go.dev/context)
- [Ethereum Keccak-256 Address Checksum Specification](https://ethereum.stackexchange.com/questions/11572/how-does-the-keccak256-hash-function-work)
