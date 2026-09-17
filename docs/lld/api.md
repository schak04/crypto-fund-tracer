# API Design

## Overview

The Go backend exposes a REST API for creating and inspecting cryptocurrency fund-flow investigations.
The API serves as the external interface for investigators and client applications, managing investigation lifecycles and persisting analysis results in PostgreSQL.

## Execution Model and Evolution

### MVP Execution Model (Synchronous)

For the MVP, investigation processing is **synchronous**:

1. The client submits a suspect address via `POST /api/v1/investigations`.
2. The backend validates the input, creates an investigation record in the database with status `running`, and immediately invokes the blockchain-analysis component.
3. The backend waits for analysis completion, persists the resulting entities (addresses, transactions, edges, attribution) to PostgreSQL, updates the investigation status to `completed` (or `failed`), and returns the full result with HTTP status `201 Created`.
4. The client can subsequently retrieve previously executed investigations using `GET /api/v1/investigations/{id}` with HTTP status `200 OK`.

### Post-MVP Planned Evolution (Asynchronous)

After the MVP is completed, investigation execution will transition to an **asynchronous** processing model to accommodate deep traces and slower blockchain data queries:

1. `POST /api/v1/investigations` will validate input, create the investigation record in status `pending`, enqueue the analysis task to a background worker, and immediately return HTTP status `202 Accepted` with a `Location: /api/v1/investigations/{id}` header.
2. The client will poll `GET /api/v1/investigations/{id}`, which returns status `pending` or `running` while in progress.
3. Once the background worker finishes, the investigation transitions to `completed` or `failed`. Subsequent calls to `GET /api/v1/investigations/{id}` return the exact same completed or failed payload defined in this specification.

The resource schemas, status enums, database entities, and internal analysis interface (`POST /analysis`) defined below are structured so that switching to asynchronous processing requires no changes to the data contracts or completed payload schemas.

## Investigation Lifecycle

```
[ POST /api/v1/investigations ]
               |
               v
            pending  (Post-MVP queued state)
               |
               v
            running  (Synchronous execution in MVP)
               |
               +--------------> failed (Completed with failure_reason)
               |
               v
           completed (Full graph and attribution persisted)
```

### Status Values

- `pending`: Investigation created and queued for processing (to be used in post-MVP async model).
- `running`: Blockchain analysis currently in progress.
- `completed`: Analysis completed successfully and all results persisted.
- `failed`: Analysis or persistence failed; failure reason recorded.

## Endpoints

### 1. Create Investigation

Synchronously creates and executes an investigation for a suspect wallet address.

```http
POST /api/v1/investigations
Content-Type: application/json
```

#### Request Headers

| Header       | Value            | Required | Description            |
| ------------ | ---------------- | -------- | ---------------------- |
| Content-Type | application/json | Yes      | Request payload format |

#### Request Body

```json
{
  "wallet_address": "0xsuspect..."
}
```

#### Request Fields

| Field          | Type   | Required | Description                                                                                                  |
| -------------- | ------ | -------- | ------------------------------------------------------------------------------------------------------------ |
| wallet_address | string | Yes      | The suspect cryptocurrency wallet address to investigate. Must match the format of the supported blockchain. |

#### Response: Success (HTTP 201 Created)

Returned when the investigation completes successfully.

Headers:

```http
Content-Type: application/json
Location: /api/v1/investigations/investigation-id-1
```

Body:

```json
{
  "id": "investigation-id-1",
  "status": "completed",
  "suspect_address": "0xsuspect...",
  "created_at": "2026-09-17T10:00:00Z",
  "completed_at": "2026-09-17T10:00:04Z",
  "failure_reason": null,
  "transactions": [
    {
      "tx_hash": "0xtx1...",
      "tx_time": "2026-09-16T18:30:00Z",
      "amount": "1.500000000000000000",
      "trace_depth": 1,
      "raw_reference": "https://explorer.../tx/0xtx1..."
    }
  ],
  "fund_flow": [
    {
      "tx_hash": "0xtx1...",
      "source_address": "0xsuspect...",
      "destination_address": "0xdestination...",
      "amount": "1.500000000000000000"
    }
  ],
  "vasp_attribution": {
    "vasp_id": "vasp-id-1",
    "name": "Binance",
    "type": "exchange",
    "matched_address": "0xdestination...",
    "attribution_status": "known"
  }
}
```

If no known exchange or VASP destination was reached during analysis, `vasp_attribution` is returned as `null`.

---

### 2. Get Investigation

Retrieves an investigation by its unique identifier.

```http
GET /api/v1/investigations/{id}
```

#### Path Parameters

| Parameter | Type            | Required | Description                             |
| --------- | --------------- | -------- | --------------------------------------- |
| id        | string (UUIDv4) | Yes      | Unique identifier of the investigation. |

#### Response: Completed Investigation (HTTP 200 OK)

Returned when the investigation exists and completed successfully. Schema is identical to the completed `POST` response.

```json
{
  "id": "investigation-id-1",
  "status": "completed",
  "suspect_address": "0xsuspect...",
  "created_at": "2026-09-17T10:00:00Z",
  "completed_at": "2026-09-17T10:00:04Z",
  "failure_reason": null,
  "transactions": [
    {
      "tx_hash": "0xtx1...",
      "tx_time": "2026-09-16T18:30:00Z",
      "amount": "1.500000000000000000",
      "trace_depth": 1,
      "raw_reference": "https://explorer.../tx/0xtx1..."
    }
  ],
  "fund_flow": [
    {
      "tx_hash": "0xtx1...",
      "source_address": "0xsuspect...",
      "destination_address": "0xdestination...",
      "amount": "1.500000000000000000"
    }
  ],
  "vasp_attribution": {
    "vasp_id": "vasp-id-1",
    "name": "Binance",
    "type": "exchange",
    "matched_address": "0xdestination...",
    "attribution_status": "known"
  }
}
```

#### Response: Failed Investigation (HTTP 200 OK)

Returned when the investigation exists in PostgreSQL but failed during analysis execution. Partial or unverified graph data is omitted to avoid inconsistent application state.

```json
{
  "id": "investigation-id-1",
  "status": "failed",
  "suspect_address": "0xsuspect...",
  "created_at": "2026-09-17T10:00:00Z",
  "completed_at": "2026-09-17T10:00:02Z",
  "failure_reason": "Blockchain data source query timed out after 30 seconds.",
  "transactions": [],
  "fund_flow": [],
  "vasp_attribution": null
}
```

#### Response: In-Progress Investigation (HTTP 200 OK - Post-MVP Async Support)

While not observed in synchronous MVP execution, this response structure is defined for future asynchronous polling:

```json
{
  "id": "investigation-id-1",
  "status": "running",
  "suspect_address": "0xsuspect...",
  "created_at": "2026-09-17T10:00:00Z",
  "completed_at": null,
  "failure_reason": null,
  "transactions": [],
  "fund_flow": [],
  "vasp_attribution": null
}
```

---

## Response Field Definitions

### Top-Level Investigation Object

| Field            | Type                 | Nullable | Description                                                                   |
| ---------------- | -------------------- | -------- | ----------------------------------------------------------------------------- |
| id               | string (UUID)        | No       | Unique identifier for the investigation.                                      |
| status           | string               | No       | Current status (`pending`, `running`, `completed`, `failed`).                 |
| suspect_address  | string               | No       | The initial suspect wallet address submitted for investigation.               |
| created_at       | string (ISO 8601)    | No       | Timestamp when investigation record was created.                              |
| completed_at     | string (ISO 8601)    | Yes      | Timestamp when processing finished (completed or failed). Null while running. |
| failure_reason   | string               | Yes      | Human-readable explanation if status is `failed`. Null otherwise.             |
| transactions     | array of Transaction | No       | List of relevant transactions discovered during fund tracing.                 |
| fund_flow        | array of FlowEdge    | No       | Directed fund transfers between addresses across traced transactions.         |
| vasp_attribution | Attribution object   | Yes      | Identified exchange/VASP details if reached; null if no match found.          |

### Transaction Object

| Field         | Type              | Nullable | Description                                                              |
| ------------- | ----------------- | -------- | ------------------------------------------------------------------------ |
| tx_hash       | string            | No       | Blockchain transaction hash.                                             |
| tx_time       | string (ISO 8601) | No       | Timestamp when the transaction was confirmed.                            |
| amount        | string            | No       | Transaction value represented as a decimal string to preserve precision. |
| trace_depth   | integer           | No       | Hop distance from the suspect address (1 = direct transfer).             |
| raw_reference | string            | Yes      | Reference URL or raw identifier for source inspection.                   |

### FlowEdge Object

| Field               | Type   | Nullable | Description                                             |
| ------------------- | ------ | -------- | ------------------------------------------------------- |
| tx_hash             | string | No       | Transaction hash associated with this transfer.         |
| source_address      | string | No       | Originating blockchain address.                         |
| destination_address | string | No       | Receiving blockchain address.                           |
| amount              | string | No       | Transferred amount along this edge as a decimal string. |

### Attribution Object

| Field              | Type          | Nullable | Description                                                             |
| ------------------ | ------------- | -------- | ----------------------------------------------------------------------- |
| vasp_id            | string (UUID) | No       | Identifier of the matched VASP entity.                                  |
| name               | string        | No       | Registered name of the VASP or exchange (e.g. Binance).                 |
| type               | string        | No       | Category of entity (e.g. `exchange`, `custodian`).                      |
| matched_address    | string        | No       | The blockchain destination address belonging to the VASP.               |
| attribution_status | string        | No       | Confidence or classification level (`known`, `probable`, `unverified`). |

---

## Error Handling

All error responses adhere to a consistent JSON error envelope.

### Error Envelope Schema

```json
{
  "error": {
    "code": "ERROR_CODE_STRING",
    "message": "Human-readable description of the error.",
    "details": [
      {
        "field": "wallet_address",
        "message": "Field is required and cannot be empty."
      }
    ]
  }
}
```

_Note: `details` is an optional array of field-level errors, included primarily on validation failures (`400 Bad Request`)._

### Error Codes and HTTP Status Mapping

| HTTP Status               | Error Code                 | Description                                                                               |
| ------------------------- | -------------------------- | ----------------------------------------------------------------------------------------- |
| 400 Bad Request           | `INVALID_REQUEST`          | Request payload is malformed JSON, unparseable, or missing required headers.              |
| 400 Bad Request           | `INVALID_ADDRESS`          | Supplied wallet address fails format or checksum validation for the supported blockchain. |
| 400 Bad Request           | `INVALID_INVESTIGATION_ID` | Path parameter `{id}` is not a valid UUIDv4 string.                                       |
| 404 Not Found             | `INVESTIGATION_NOT_FOUND`  | No investigation exists with the specified UUID.                                          |
| 502 Bad Gateway           | `ANALYSIS_SERVICE_ERROR`   | External blockchain analysis service or data source returned an error or timed out.       |
| 500 Internal Server Error | `INTERNAL_ERROR`           | Unexpected application failure (e.g. database error, unhandled exception).                |

### Error Examples

#### 1. Invalid Address Format (HTTP 400 Bad Request)

```json
{
  "error": {
    "code": "INVALID_ADDRESS",
    "message": "The supplied wallet address is not a valid address format for the supported blockchain.",
    "details": [
      {
        "field": "wallet_address",
        "message": "Supplied address fails checksum or length validation."
      }
    ]
  }
}
```

#### 2. Investigation Not Found (HTTP 404 Not Found)

```json
{
  "error": {
    "code": "INVESTIGATION_NOT_FOUND",
    "message": "Investigation with ID 'investigation-id-1' does not exist."
  }
}
```

#### 3. External Analysis Failure (HTTP 502 Bad Gateway)

Occurs during synchronous MVP execution when the analysis component or blockchain RPC fails. The backend records the investigation in `failed` status and returns:

```json
{
  "error": {
    "code": "ANALYSIS_SERVICE_ERROR",
    "message": "Blockchain analysis component failed to complete fund tracing: provider timeout.",
    "investigation_id": "investigation-id-1"
  }
}
```

---

## Backend to Blockchain Analysis Contract

The Go backend coordinates with the blockchain-analysis component over an internal language-independent HTTP interface.

### Analysis Endpoint

```http
POST /analysis
Content-Type: application/json
```

#### Analysis Request Payload

```json
{
  "wallet_address": "0xsuspect...",
  "max_depth": 3
}
```

| Field          | Type    | Required | Description                                                         |
| -------------- | ------- | -------- | ------------------------------------------------------------------- |
| wallet_address | string  | Yes      | Suspect blockchain address to trace.                                |
| max_depth      | integer | No       | Maximum traversal depth from suspect address. Defaults to 3 in MVP. |

#### Analysis Response Payload (HTTP 200 OK)

```json
{
  "wallet_address": "0xsuspect...",
  "addresses": [
    {
      "address": "0xsuspect...",
      "role": "suspect"
    },
    {
      "address": "0xdestination...",
      "role": "destination"
    }
  ],
  "transactions": [
    {
      "tx_hash": "0xtx1...",
      "tx_time": "2026-09-16T18:30:00Z",
      "amount": "1.500000000000000000",
      "trace_depth": 1,
      "raw_reference": "https://explorer.../tx/0xtx1..."
    }
  ],
  "fund_flow": [
    {
      "tx_hash": "0xtx1...",
      "source_address": "0xsuspect...",
      "destination_address": "0xdestination...",
      "amount": "1.500000000000000000"
    }
  ],
  "vasp_attribution": {
    "name": "Binance",
    "type": "exchange",
    "matched_address": "0xdestination...",
    "attribution_status": "known"
  }
}
```

The backend maps the received `addresses`, `transactions`, `fund_flow`, and `vasp_attribution` directly into the corresponding PostgreSQL entities defined in [database.md](database.md).

#### Analysis Failure Response (HTTP 500 / 502)

```json
{
  "error": {
    "code": "ANALYSIS_FAILED",
    "message": "Failed to fetch transactions for block range: node RPC timeout."
  }
}
```

---

## Validation

The backend performs validation before executing analysis:

- Request body must be valid, well-formed JSON.
- `wallet_address` is mandatory, non-empty, and conforms to the address format and checksum rules of the supported blockchain.
- Investigation ID parameters on `GET` requests must be valid UUIDv4 strings.
- Max depth (if supplied) must be a positive integer within configured application boundaries (1 to 5).

---

## API Design Principles

### Version the API

The prefix `/api/v1` is maintained across all endpoints to allow evolving the API without breaking existing consumers.

### Investigator-Oriented Representation

Responses provide normalized, application-level fund-flow graphs and attribution data rather than exposing raw, unstructured node or explorer RPC data.

### Future-Proof Contracts

Response schemas and internal analysis interfaces remain consistent between the synchronous MVP and the planned asynchronous queue-based execution model.
