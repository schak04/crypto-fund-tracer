# API Design

## Overview

The Go backend exposes a REST API for creating and inspecting blockchain investigations.
The API is responsible for application-level operations.

## API Responsibilities

The API should:

- accept investigation requests
- validate input
- create investigations
- trigger blockchain analysis
- return investigation status
- return completed investigation results
- return appropriate errors

## Endpoints

### Create Investigation

```http
POST /api/v1/investigations
Content-Type: application/json
```

Request:

```json
{
  "wallet_address": "..."
}
```

The backend validates the address before starting the investigation.

Example response:

```json
{
  "id": "investigation-id",
  "status": "pending"
}
```

### Get Investigation

```http
GET /api/v1/investigations/{id}
```

Example response while processing:

```json
{
  "id": "investigation-id",
  "status": "running"
}
```

Example completed response:

```json
{
  "id": "investigation-id",
  "status": "completed",
  "suspect_address": "...",
  "transactions": [],
  "fund_flow": [],
  "vasp_attribution": null
}
```

The exact response structure will be finalised after the blockchain-analysis result contract is defined.

## Investigation Lifecycle

```
POST /api/v1/investigations
           |
           v
        pending
           |
           v
        running
           |
           +------> failed
           |
           v
       completed
```

## Backend to Blockchain Analysis Interface

The application backend should communicate with the blockchain-analysis component through a language-independent contract.

Conceptually:

```http
POST /analysis
Content-Type: application/json
```

Request:

```json
{
  "wallet_address": "...",
  "max_depth": 3
}
```

The analysis component returns structured data.

Example:

```json
{
  "wallet_address": "...",
  "addresses": [],
  "transactions": [],
  "fund_flow": [],
  "vasp_attribution": null
}
```

The exact fields are intentionally not final at this stage.

They depend on:

- selected blockchain
- transaction model
- selected data source
- tracing algorithm
- attribution approach

## Error Handling

The API should use appropriate HTTP status codes.

Examples:

```
400 Bad Request
```

Invalid request data or wallet address.

```
404 Not Found
```

Investigation does not exist.

```
409 Conflict
```

Request conflicts with an existing investigation where applicable.

```
502 Bad Gateway
```

A required external analysis/data service failed.

```
500 Internal Server Error
```

Unexpected application failure.

The API should return structured error responses.

Example:

```json
{
  "error": {
    "code": "INVALID_ADDRESS",
    "message": "The supplied wallet address is invalid."
  }
}
```

## Validation

The backend should validate:

- required fields
- wallet address format
- supported blockchain/address type
- request limits

Blockchain-specific validation rules should be kept separate from general application validation where practical.

## API Design Principles

### Version the API

Use:

```
/api/v1
```

This allows the API to evolve without immediately breaking existing clients.

### Responses should be kept application-oriented

The REST API should expose information useful to the investigator rather than simply returning raw responses from a blockchain API.

### The blockchain component should be kept independent

The Go backend should not depend on the implementation language of the blockchain-analysis component.
Only the agreed data contract should matter.

### The MVP should be kept small

The initial API only needs enough functionality to:

1. create an investigation
2. inspect its status
3. retrieve its result

Authentication, user management, pagination, advanced filtering, continuous monitoring, and other application features are outside the initial MVP unless they become necessary.
