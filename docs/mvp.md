# MVP

## Goal

To build a working intial version that can take a cryptocurrency wallet address reported as suspicious, trace its relevant transactions on one supported blockchain, and determine whether the funds reach a known cryptocurrency exchange or VASP.

## MVP Scope

The system will initially support:

- One blockchain ecosystem
- One suitable blockchain data source/API
- User-provided suspect wallet address
- Retrieval of relevant transactions
- Basic transaction parsing and normalization
- Limited-depth fund-flow tracing
- Transaction graph representation
- A small dataset of known exchange/VASP addresses
- Basic exchange/VASP attribution
- Investigation result generation
- PostgreSQL persistence
- Go REST API
- Basic automated tests

The initial interface can be an API or a simple command-line one. A polished frontend can wait until the core MVP work is done.

## Core Workflow

```
Suspect Wallet Address
      |
      v
  Go Backend
      |
      v
Blockchain Analysis
      |
      | (retrieves transaction data
      |  from the blockchain data source)
      v
Transaction Data (-> parsed and normalized)
      |
      v
Fund-Flow Tracing
      |
      v
Known Exchange/VASP Data (compared with traced destinations)
      |
      v
Investigation Result
      |
      |-> exposed through the API
      |
      |-> persisted (PostgreSQL)
```

The result should contain enough information for an investigator to understand:

- the submitted wallet address
- relevant transactions
- how funds moved from the address
- important intermediary addresses
- whether a known exchange/VASP was reached
- the relevant transaction details supporting the attribution

## Deliberate MVP Limitations

The MVP will not attempt to support:

- multiple blockchains
- cross-chain tracing
- bridges
- complex DeFi analysis
- mixers or tumblers
- privacy-enhancing mechanisms
- large-scale blockchain indexing
- comprehensive exchange wallet clustering
- machine learning
- advanced fraud classification
- real-time continuous monitoring
- NCRP integration
- SAHYOG integration
- production-scale deployment

These are future improvements, not requirements for the first working version.

## What Makes the MVP Complete?

The MVP is complete when the following workflow works end to end:

```
    Submit a suspect wallet address
                  |
                  v
        Retrieve blockchain data
                  |
                  v
        Trace relevant transactions
                  |
                  v
       Identify the resulting flow
                  |
                  v
     Match known exchange/VASP data
                  |
                  v
       Store and return the result
```

## Responsibility Split

### Backend and Application Layer

Primary responsibility:

- Go backend
- REST API
- PostgreSQL
- investigation management
- persistence
- integration with blockchain analysis
- API-level testing
- overall application integration

### Blockchain Analysis

Primary responsibility:

- blockchain data acquisition
- transaction parsing
- transaction normalization
- fund-flow traversal
- transaction graph construction
- known exchange/VASP address dataset
- basic attribution logic
- blockchain-analysis testing

### Frontend

The frontend is intentionally deferred until the core system is functional.  
It should consume the existing backend API and present the investigation results in a simple investigator-friendly interface.

> [!NOTE]
>
> The MVP should prove the core concept with the smallest reasonable implementation.  
> If a feature does not contribute directly to:  
> `wallet -> blockchain transactions -> fund flow -> exchange/VASP attribution`  
> it should not be required for the MVP.
