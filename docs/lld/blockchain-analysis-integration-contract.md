# Blockchain Analysis Integration Contract

## 1. Overview and Purpose

This document defines the language-independent integration contract between the Go backend (`crypto-fund-tracer`) and the Blockchain Analysis component.
It establishes the data schemas, transport protocols, depth semantics, entity representations, error behaviours, and architectural boundaries required to implement the blockchain analysis layer.

---

## 2. Boundaries and Architectural Assumptions

### Go Backend Responsibilities

- Exposes public REST endpoints to investigators and client applications.
- Validates request inputs (e.g. wallet address format and checksums, parameter types).
- Creates initial investigation records in PostgreSQL with status `running`.
- Triggers the blockchain analysis component via HTTP `POST /analysis`.
- Atomically persists parsed transactions, addresses, fund-flow edges, and attribution records into PostgreSQL.
- Updates investigation status to `completed` upon success, or `failed` upon failure.
- Manages application configuration, client connection timeouts, and audit logs.

### Blockchain Analysis Component Responsibilities

- Implements an HTTP server listening for `POST /analysis`.
- Connects to the supported blockchain network data provider (RPC node, indexing service, or explorer API).
- Parses raw transactions into normalized structures.
- Traverses outgoing transactions from the suspect wallet up to the requested `max_depth`.
- Assigns roles to discovered addresses (`suspect`, `intermediary`, `destination`, `other`).
- Compares destination addresses against a dataset of known cryptocurrency exchange / VASP addresses.
- Returns the complete graph and attribution payload as JSON on HTTP 200 OK, or structured error JSON on failure.

### Explicit Boundary Constraints

1. **No Database Access**: The blockchain analysis component does not connect to PostgreSQL. All persistence is owned exclusively by the Go backend.
2. **Stateless Execution**: Each `POST /analysis` invocation is self-contained. The analysis component does not need to store state across requests.
3. **No Database UUIDs**: The analysis component identifies entities solely by their on-chain identifiers (wallet addresses and transaction hashes). Database UUID generation and foreign keys are handled by the backend repository.
4. **Timeout Expectation**: For synchronous MVP execution, the analysis component must complete or return an error within 30 seconds.

---

## 3. Transport Protocol and Endpoint Specification

- **Protocol**: HTTP/1.1
- **Method**: `POST`
- **Path**: `/analysis`
- **Request Header**: `Content-Type: application/json`
- **Response Header**: `Content-Type: application/json`

---

## 4. Request / Input Structure

> [!IMPORTANT]  
> The wallet addresses and transaction hashes shown in the schemas and examples below are synthetic sample fixtures and public test vectors (including canonical EIP-55 fixtures and public exchange addresses). They are used solely for format illustration and local integration testing, and do not represent active investigations or private personal data.

### Schema

```json
{
  "wallet_address": "0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed",
  "max_depth": 3
}
```

### Field Definitions

| Field            | Type    | Required | Default | Description                                                                                          |
| ---------------- | ------- | -------- | ------- | ---------------------------------------------------------------------------------------------------- |
| `wallet_address` | string  | Yes      | None    | The suspect blockchain address to trace. For EVM/Ethereum, must be 42 characters starting with `0x`. |
| `max_depth`      | integer | No       | 3       | Traversal hop distance limit. Must be a positive integer between 1 and 5.                            |

---

## 5. Depth and Traversal Semantics

- **Depth 0**: The suspect address itself.
- **Depth 1**: Direct outgoing transactions where `source_address` is the suspect wallet.
- **Depth 2**: Outgoing transactions from recipients of Depth 1 transfers.
- **Depth N**: Outgoing transactions originating from recipients at Depth N-1.
- **Traversal Halting Conditions**:
  1. The traversal reaches `max_depth`.
  2. A destination address matches a known exchange or VASP entity.
  3. An address has no outgoing transactions (terminal wallet).
  4. Circular flows / loops: if funds return to an address already visited in the current trace path, the cycle must not cause an infinite loop.

---

## 6. Response / Output Structure (HTTP 200 OK)

### Success Payload Schema

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

If no known exchange or VASP destination was reached during analysis, `vasp_attribution` must be returned as `null`.

---

## 7. Entity Field Definitions and Data Precision

### Top-Level Object

| Field              | Type                     | Nullable | Description                                                              |
| ------------------ | ------------------------ | -------- | ------------------------------------------------------------------------ |
| `wallet_address`   | string                   | No       | The suspect wallet address submitted in the request.                     |
| `addresses`        | array of AddressItem     | No       | List of unique addresses observed in the traced flow.                    |
| `transactions`     | array of TransactionItem | No       | List of transactions included in the trace graph.                        |
| `fund_flow`        | array of FlowEdgeItem    | No       | Directed fund transfer hops connecting addresses.                        |
| `vasp_attribution` | VASPAttributionItem      | Yes      | Attribution information if an exchange/VASP was reached; null otherwise. |

### AddressItem

| Field     | Type   | Allowed Values                                    | Description                                                               |
| --------- | ------ | ------------------------------------------------- | ------------------------------------------------------------------------- |
| `address` | string | Valid blockchain address                          | The normalized blockchain address string.                                 |
| `role`    | string | `suspect`, `intermediary`, `destination`, `other` | Functional classification of this address within the investigation graph. |

- `suspect`: The initial suspect wallet address submitted for investigation.
- `intermediary`: A pass-through wallet between the suspect address and the final destination.
- `destination`: The terminal receiving address where traced funds stopped or entered an exchange.
- `other`: Ancillary observed wallet (e.g. fee payer or contract).

### TransactionItem

| Field           | Type    | Format / Constraints                  | Description                                                                |
| --------------- | ------- | ------------------------------------- | -------------------------------------------------------------------------- |
| `tx_hash`       | string  | Unique blockchain hash                | Primary transaction identifier (e.g. 0x...).                               |
| `tx_time`       | string  | ISO 8601 UTC (`YYYY-MM-DDTHH:MM:SSZ`) | Block confirmation timestamp.                                              |
| `amount`        | string  | Numeric decimal string                | Transaction value. Decimal string format prevents IEEE 754 precision loss. |
| `trace_depth`   | integer | 1 to 5                                | Hop distance from the suspect address at which this transaction occurred.  |
| `raw_reference` | string  | Valid URL or block/tx reference       | Link to public explorer or raw data reference for auditing.                |

### FlowEdgeItem

| Field                 | Type   | Description                                                           |
| --------------------- | ------ | --------------------------------------------------------------------- |
| `tx_hash`             | string | Transaction hash associated with this transfer.                       |
| `source_address`      | string | Originating address for this transfer hop. Must exist in `addresses`. |
| `destination_address` | string | Receiving address for this transfer hop. Must exist in `addresses`.   |
| `amount`              | string | Amount transferred along this specific edge as a decimal string.      |

### VASPAttributionItem

| Field                | Type   | Allowed Values / Examples                               | Description                                                     |
| -------------------- | ------ | ------------------------------------------------------- | --------------------------------------------------------------- |
| `name`               | string | e.g. `"Binance"`, `"Coinbase"`, `"Kraken"`              | Registered or common name of the entity.                        |
| `type`               | string | e.g. `"exchange"`, `"custodian"`, `"payment_processor"` | Functional entity category.                                     |
| `matched_address`    | string | Must match one `destination` address                    | The destination address identified as belonging to this entity. |
| `attribution_status` | string | `known`, `probable`, `unverified`                       | Confidence level of the address attribution.                    |

---

## 8. Errors and Failure Behaviour

When the analysis component cannot complete fund tracing, it must return an appropriate HTTP error status code with a JSON error payload.

### Error Envelope Schema

```json
{
  "error": {
    "code": "ERROR_CODE_STRING",
    "message": "Human-readable description of what failed."
  }
}
```

### Standard Error Codes and Status Mappings

| HTTP Status               | Error Code        | Description                                                                                    |
| ------------------------- | ----------------- | ---------------------------------------------------------------------------------------------- |
| 400 Bad Request           | `INVALID_REQUEST` | Payload is unparseable or parameters (such as address format or depth bounds) are invalid.     |
| 500 Internal Server Error | `ANALYSIS_FAILED` | Internal tracing error, parsing failure, or unhandled exception within the analysis component. |
| 502 Bad Gateway           | `RPC_TIMEOUT`     | Upstream blockchain node or explorer API timed out or refused connection.                      |
| 502 Bad Gateway           | `RATE_LIMITED`    | Upstream blockchain data provider rate limit exceeded.                                         |

### Example Error Payloads

#### Upstream Provider Timeout (HTTP 502)

```json
{
  "error": {
    "code": "RPC_TIMEOUT",
    "message": "Failed to fetch transaction history: node RPC request timed out after 20 seconds."
  }
}
```

#### Parsing Failure (HTTP 500)

```json
{
  "error": {
    "code": "ANALYSIS_FAILED",
    "message": "Failed to decode transaction input payload for transaction 0xabc...: unsupported ABI format."
  }
}
```

### Backend Failure Handling

When the Go backend receives an error response or encounters an HTTP connection timeout:

1. It records the investigation status as `failed` in PostgreSQL.
2. It sets `completed_at` to the current timestamp.
3. It stores the error message into `failure_reason`.
4. It responds to the client with HTTP status `502 Bad Gateway` and error code `ANALYSIS_SERVICE_ERROR`, including the `investigation_id`.

---

## 9. Implementation Checklist for Blockchain Analysis

To verify integration readiness with the Go backend:

- [ ] HTTP server accepts `POST /analysis` with `Content-Type: application/json`.
- [ ] Returns HTTP 200 with all top-level keys (`wallet_address`, `addresses`, `transactions`, `fund_flow`, `vasp_attribution`).
- [ ] `wallet_address` echoes the queried suspect address (the blockchain analysis response must include the same wallet address that Go sent in the request).
- [ ] Every address in `fund_flow` edges (`source_address` and `destination_address`) is included in `addresses`.
- [ ] The suspect address is included in `addresses` with role `suspect`.
- [ ] Monetary amounts are serialized as decimal strings (never as floats or raw integers).
- [ ] Timestamps use standard ISO 8601 UTC format (`2026-09-16T18:30:00Z`).
- [ ] `vasp_attribution` is either a valid object or explicitly `null`.
- [ ] Errors return a JSON body adhering to `{"error": {"code": "...", "message": "..."}}`.
- [ ] Total execution time remains within the 30-second client timeout window.
