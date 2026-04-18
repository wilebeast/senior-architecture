# AtlasX Exchange Core Architecture

## 1. Business Positioning

This project is designed as a `principal architect portfolio case` for a Web3 digital currency exchange. The focus is not only on writing code, but on demonstrating the architecture thinking expected from the target role:

- exchange domain decomposition
- low-latency trading path design
- wallet and ledger consistency
- operational resilience
- cloud-native scaling
- security by design
- regulatory readiness

## 2. Domain Scope

The platform is modeled as a spot exchange with the following bounded contexts:

1. `API Gateway`
Receives authenticated requests, rate limits them, performs idempotency checks, and routes traffic to internal services.

2. `Identity / KYC / AML`
Owns user onboarding, sanctions screening, transaction monitoring, and suspicious activity workflows.

3. `Risk Engine`
Performs pre-trade validation, account state checks, symbol constraints, limits, and policy controls.

4. `Matching Engine`
Runs the low-latency price-time-priority order book and generates executions.

5. `Wallet & Ledger`
Owns balances, reservations, settlements, deposit/withdraw states, and auditable accounting.

6. `Market Data`
Builds order book snapshots, trade ticks, candles, and downstream streams.

7. `Clearing & Settlement`
Applies deterministic post-trade asset movements and fee calculations.

8. `Security & Key Management`
Owns key rotation, signing policies, hot/cold wallet controls, HSM/KMS integration, and break-glass procedures.

9. `Audit & Compliance Reporting`
Stores immutable business events and exposes regulator-friendly trails.

## 3. Architecture Strategy

### Prototype Form

The current repository uses a `modular monolith` in Go because that is the fastest way to demonstrate correctness, interfaces, and critical trading flow inside one repo.

### Production Evolution

In production, the system should separate into these planes:

- `control plane`: user, KYC, AML, admin, configuration, reporting
- `trading plane`: order entry, risk, matching, market data
- `asset plane`: wallet, blockchain connectivity, withdrawals, treasury
- `governance plane`: audit, observability, policy, secrets, compliance

The matching path stays as small as possible:

`gateway -> risk -> funds reservation -> matching -> settlement -> market data fan-out`

Everything non-critical is pushed off the latency path.

## 4. Request And Event Flow

### 4.1 Order Submission

1. Client sends signed order to `API Gateway`
2. Gateway authenticates, validates schema, rate limits, applies idempotency token
3. Risk engine checks:
   - account status
   - symbol status
   - precision / lot size / tick size
   - available balance
   - limits / circuit breaker / velocity checks
4. Wallet service reserves required funds
5. Matching engine matches against opposite book using price-time priority
6. Settlement applies deterministic balance changes
7. Market data updates snapshots and trade tape
8. Audit service records business events

### 4.2 Deposits And Withdrawals

Production design:

- deposit watchers subscribe to chain events
- confirmations are tracked by chain-specific finality rules
- credit events go through ledger, not direct balance mutation
- withdrawals pass policy checks, AML scoring, address screening, and approval workflow
- hot wallet service signs only policy-approved transactions
- treasury periodically rebalances hot/cold inventory

## 5. Low-Latency Design Principles

The `matching engine` is the most latency-sensitive component.

Principles:

- single writer per symbol partition
- no remote calls in the core matching loop
- pre-validated and pre-reserved funds before entering the engine
- append-only event generation
- deterministic state transitions
- lock minimization and memory-local data structures

Scaling model:

- shard symbols across engine partitions
- pin partitions to dedicated CPU resources
- use replicated read models for market data and reporting
- isolate retail APIs from internal market-data and admin traffic

## 6. Data Model And Consistency

The system intentionally separates:

- `available balance`
- `locked balance`
- `ledger events`
- `order state`
- `trade executions`

This is critical for preventing asset drift.

Consistency approach:

- pre-trade reservation is synchronous
- match events are deterministic
- settlement is derived from executions
- audit events are append-only

Production improvement:

- persist order / ledger / trade events to Kafka or Redpanda
- snapshot engine state periodically
- support deterministic replay for recovery and audit

Current repository upgrade:

- `PostgreSQL` persists balances, orders, trades, and audit events
- `Redis` caches account snapshots, books, and trades
- `Redpanda` publishes order / trade / audit events
- service bootstrap restores balances and open orders into runtime memory

## 7. Security Architecture

### 7.1 Layered Defense

- WAF and API rate limiting
- zero-trust service-to-service identity
- mTLS between internal services
- KMS/HSM-backed key storage
- strict secret rotation
- environment isolation between trading and wallet planes
- immutable audit trail

### 7.2 Wallet Controls

- hot wallet for limited operational liquidity
- cold wallet for treasury reserve
- warm wallet optional for controlled rebalancing
- threshold signing / MPC for high-value movement
- withdrawal allowlist and policy engine
- transaction simulation and anomaly checks before signing

### 7.3 Application Security

- RBAC for admin functions
- least-privilege IAM
- secure SDLC and dependency scanning
- replay protection for trade APIs
- request signing and nonce windows
- tamper-evident audit logging

## 8. Compliance Readiness

The JD explicitly calls out `AML / CFT`, so the architecture includes:

- KYC onboarding states
- sanctions / PEP screening integration
- travel rule integration point
- transaction monitoring rules engine
- suspicious activity case management
- jurisdiction-specific policy configuration
- asset provenance and address risk scoring hooks

The prototype does not implement external compliance vendors, but it exposes the correct domain seams.

## 9. Cloud And High Availability

Recommended production deployment on `AWS`:

- `EKS` for control and API services
- dedicated low-latency node groups for trading partitions
- `MSK` or self-managed Kafka / Redpanda for event streaming
- `Aurora PostgreSQL` for control plane relational data
- `Redis` for low-latency cache and session controls
- `S3` for snapshots, archives, and audit export
- `KMS` / `CloudHSM` for key management
- `Prometheus + Grafana + Loki + Tempo` for observability

Local developer stack in this repository:

- `docker-compose.yml` runs `exchange + postgres + redis + redpanda`
- schema is applied automatically by the exchange service on startup
- local persistence enables restart recovery for balances and open orders

HA strategy:

- active-active control plane across AZs
- active-standby or partitioned-active engine deployment by symbol group
- RPO minimized through event log replication
- recovery by snapshot + event replay

## 10. Engineering Standards

This project demonstrates the standards an architect should enforce:

- explicit bounded contexts
- minimal and testable interfaces
- deterministic business logic
- immutable domain events for auditability
- clear separation of latency-critical and non-critical flows
- operational documentation as a first-class artifact

## 11. Prototype Limitations

This repository is intentionally a `portfolio prototype`, not a production exchange. It does not include:

- real blockchain connectivity
- persistent storage
- authentication / authorization
- distributed event bus
- matching partitioning across processes
- full fee engine
- derivatives / liquidation logic

Those are the next steps after proving the core architecture and domain design.
