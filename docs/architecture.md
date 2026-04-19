# AtlasX Exchange Core Architecture

## 1. Document Status

This document now distinguishes between:

- `Implemented`: present in the current repository
- `Partially Implemented`: core seam exists, but production features are incomplete
- `Planned`: architecture target, not implemented in this repository

Current repository position:

- `Prototype form`: implemented
- `Stateful local stack`: implemented
- `Production-grade exchange controls`: mostly planned

## 2. Business Positioning

This project is a `principal architect portfolio case` for a Web3 digital asset exchange. Its purpose is to demonstrate:

- exchange domain decomposition
- low-latency trading path design
- wallet and balance consistency
- persistence and event-driven recovery
- operational validation through local infrastructure
- awareness of security, compliance, and cloud production requirements

It is not a full production exchange.

## 3. Current Implementation Summary

### Implemented

- spot order entry API
- price-time-priority in-memory matching engine
- pre-trade balance reservation
- deterministic settlement between buyer and seller
- account balances with `available` and `locked`
- PostgreSQL persistence for balances, orders, trades, and audit events
- Redis cache for accounts, order books, and trades
- Redpanda publishing for order, trade, and audit events
- restart recovery for open orders and balances
- audit event recording
- Docker Compose local stack
- smoke / restart / inspection / acceptance scripts

### Partially Implemented

- API gateway behavior
  The HTTP layer exists, but there is no auth, signing, idempotency, or rate limiting.
- wallet and ledger
  Balances, reservations, and settlement exist, but there is no real blockchain integration or treasury workflow.
- market data
  Trades and book snapshots exist, but no candles, streams, or downstream fan-out service.
- recovery model
  Bootstrap recovery exists, but not snapshotting plus deterministic replay.
- event-driven architecture
  Redpanda publish exists, but there are no consumers, outbox, or independently deployed services.

### Planned

- KYC / AML domain
- withdrawals and chain listeners
- hot / warm / cold wallet controls
- HSM / KMS integration
- fee engine
- real API gateway controls
- RBAC / IAM / admin control plane
- multi-process matching partitions
- cloud-native production deployment
- observability and SRE stack

## 4. Domain Scope

The target platform is modeled as a spot exchange with these bounded contexts.

### 4.1 API Gateway

Status: `Partially Implemented`

Current repository:

- HTTP endpoints for deposit, order placement, account lookup, order book lookup, trades, and audit

Not implemented:

- authentication
- authorization
- rate limiting
- idempotency keys
- signed requests

### 4.2 Identity / KYC / AML

Status: `Planned`

Target responsibility:

- onboarding
- sanctions screening
- transaction monitoring
- suspicious activity workflows

Current repository:

- no implementation

### 4.3 Risk Engine

Status: `Partially Implemented`

Current repository:

- basic price/quantity validation
- symbol validation
- available balance checks

Not implemented:

- circuit breakers
- position or velocity limits
- jurisdiction policies
- account risk states

### 4.4 Matching Engine

Status: `Implemented`

Current repository:

- price-time-priority order book
- deterministic in-memory matching
- per-symbol order book state
- trade generation with `executed_at`

Not implemented:

- partitioned engine workers
- dedicated low-latency process isolation
- derivatives logic

### 4.5 Wallet & Ledger

Status: `Partially Implemented`

Current repository:

- balances with `available` and `locked`
- reservation for open orders
- settlement after execution
- recovery-time lock rebuild for open orders

Not implemented:

- double-entry ledger service
- blockchain deposit/withdraw flow
- treasury operations
- reconciliation against chain state

### 4.6 Market Data

Status: `Partially Implemented`

Current repository:

- order book snapshots
- trade history
- Redis cache for hot reads

Not implemented:

- candle generation
- WebSocket streaming
- dedicated market data fan-out service

### 4.7 Clearing & Settlement

Status: `Partially Implemented`

Current repository:

- deterministic spot settlement inside exchange service

Not implemented:

- separate clearing service
- fees, rebates, commissions
- reconciliation workflows

### 4.8 Security & Key Management

Status: `Planned`

Current repository:

- only basic local environment configuration

Not implemented:

- secret rotation
- HSM / KMS
- signing policy engine
- wallet key hierarchy
- MPC / threshold signing

### 4.9 Audit & Compliance Reporting

Status: `Partially Implemented`

Current repository:

- append-style audit event persistence
- audit event publishing to Redpanda
- audit query endpoint

Not implemented:

- regulator-oriented reporting
- case workflows
- retention and archival policy

## 5. Architecture Strategy

### Prototype Form

Status: `Implemented`

The repository uses a `modular monolith` in Go. This is intentional:

- fast to implement
- easy to validate locally
- keeps critical trading flow visible in one codebase

### Production Evolution

Status: `Planned`

Production separation should evolve toward these planes:

- `control plane`: user, KYC, AML, admin, reporting
- `trading plane`: order entry, risk, matching, market data
- `asset plane`: wallet, blockchain connectivity, withdrawals, treasury
- `governance plane`: audit, observability, policy, secrets, compliance

Current repository does not implement this decomposition as separately deployed services.

## 6. Request And Event Flow

### 6.1 Order Submission

Status: `Partially Implemented`

Implemented flow:

1. Client sends HTTP request
2. API layer validates request payload
3. Risk engine validates order and balance sufficiency
4. Wallet reserves funds
5. Matching engine matches by price-time priority
6. Settlement mutates balances
7. Trades and orders are persisted
8. Cache and event stream are updated
9. Audit event is recorded

Planned but not implemented:

- signed order requests
- rate limiting
- idempotency token handling
- asynchronous consumer services

### 6.2 Deposits And Withdrawals

Status: `Partially Implemented`

Implemented:

- manual deposit API that credits balances

Planned:

- chain watchers
- finality tracking
- withdrawal approval workflow
- sanctions and address screening
- hot/cold wallet movement

## 7. Low-Latency Design

Status: `Partially Implemented`

Implemented design choices:

- in-memory order book
- pre-reserved funds before entering matching
- deterministic matching and settlement flow
- small critical path inside one process

Planned design choices:

- single-writer partition ownership by symbol shard
- CPU pinning
- isolated engine processes
- independent read models and fan-out services

## 8. Data Model And Consistency

Status: `Partially Implemented`

Implemented:

- separate `available` vs `locked` balances
- explicit `order state`
- explicit `trade executions`
- persisted `audit events`
- PostgreSQL as durable state store
- Redis as hot read cache
- Redpanda as event publication channel
- bootstrap recovery for balances and open orders

Not yet implemented:

- dedicated ledger event store
- snapshotting plus replay
- outbox pattern
- consumer-driven read model rebuild

## 9. Security Architecture

Status: `Planned`

The repository does not implement production-grade security controls. The following remain target-state architecture:

- WAF
- zero-trust service identity
- mTLS
- KMS / HSM integration
- least-privilege IAM
- replay-protected trade API signing
- admin RBAC
- tamper-evident archival pipeline

## 10. Compliance Readiness

Status: `Planned`

The architecture anticipates:

- KYC states
- AML / CFT hooks
- sanctions / PEP screening
- travel rule integration
- suspicious activity workflows
- jurisdiction-specific policy

Current repository:

- no actual compliance service implementation

## 11. Cloud And High Availability

Status: `Partially Implemented`

Implemented locally:

- `docker-compose.yml` local stack
- PostgreSQL / Redis / Redpanda integration
- restart recovery for balances and open orders
- acceptance scripts for operational validation

Planned production deployment:

- `EKS`
- `Aurora PostgreSQL`
- managed or dedicated Kafka / Redpanda
- object storage for snapshots and archives
- observability stack such as Prometheus / Grafana / Loki / Tempo
- multi-AZ active-active / active-standby topology

Not implemented:

- Kubernetes manifests
- cloud infra as code
- multi-region failover
- RPO/RTO automation

## 12. Engineering Standards

Status: `Implemented`

This repository demonstrates:

- explicit domain separation in code structure
- small interfaces for store, cache, and event bus
- deterministic matching flow
- operational scripts for validation
- documentation that maps architecture to implementation

## 13. Prototype Limitations

This repository is still a `portfolio prototype`.

Not implemented:

- real blockchain connectivity
- KYC / AML services
- authentication / authorization
- fee engine
- advanced market data products
- distributed matching partitions
- derivatives / liquidation logic
- HSM / MPC wallet controls
- production cloud deployment stack

Already implemented despite earlier prototype wording:

- persistent storage through PostgreSQL
- event publication through Redpanda
- Redis-backed read cache
- restart-time recovery of balances and open orders

## 14. Recommended Next Steps

If the goal is to move this prototype closer to a real exchange core, the next useful increments are:

1. add auth, idempotency, and rate limiting at the API layer
2. introduce outbox + consumer workers for stronger event consistency
3. add a real ledger event model and fee engine
4. add withdrawal / wallet control workflows
5. split matching, wallet, and market data into independently deployable services
