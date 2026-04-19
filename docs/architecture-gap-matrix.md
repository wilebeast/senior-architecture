# Architecture Gap Matrix

This matrix maps the target JD to the current repository state.

Legend:

- `Implemented`
- `Partially Implemented`
- `Planned`

| JD Requirement | Current State | Current Implementation | Gap | Next Step |
| --- | --- | --- | --- | --- |
| Overall exchange architecture | Partially Implemented | Matching, wallet, persistence, cache, event bus, audit, local stack | No real control plane / asset plane / governance plane split | Break monolith into service boundaries and add outbox/consumers |
| Matching engine | Implemented | In-memory price-time-priority orderbook with deterministic settlement | Single-process only | Add partitioned engine workers and isolated processes |
| Wallet system | Partially Implemented | Available/locked balances, reservation, recovery-time lock rebuild | No blockchain connectivity, treasury, reconciliation | Add double-entry ledger and deposit/withdraw workflows |
| Clearing and settlement | Partially Implemented | Spot settlement performed synchronously in exchange service | No fee engine or separate clearing service | Add ledger events, fees, reconciliation jobs |
| Market data system | Partially Implemented | Orderbook snapshots and trade history, Redis cache | No streaming, candles, downstream fan-out | Add WebSocket streams and candle aggregation |
| API gateway | Partially Implemented | HTTP API endpoints for core spot workflow | No auth, signing, idempotency, rate limiting | Add authn/authz, request signing, limiter, idempotency keys |
| KYC / AML | Planned | None | Entire domain missing | Add onboarding state model and compliance service stubs |
| High concurrency / low latency | Partially Implemented | Small in-memory matching path, pre-reserved balances | No process isolation, no sharding, no benchmarked low-latency infra | Add symbol partitioning and benchmark harness |
| Massive data processing / storage | Partially Implemented | PostgreSQL persistence, Redis cache, Redpanda events | No archival, no snapshots, no replay pipeline | Add snapshot + replay and object storage export |
| Scalability / elasticity | Planned | Local Docker Compose only | No autoscaling or cloud deployment topology | Add Kubernetes deployment and service decomposition |
| Security architecture | Planned | Basic local config only | No mTLS, KMS/HSM, IAM, WAF, signing controls | Add security control doc + initial auth/secrets implementation |
| Hot / cold wallet / key management | Planned | None | Entire wallet security plane missing | Add wallet domain, approval workflow, KMS/HSM abstraction |
| Compliance with AML / CFT | Planned | None | No policy engine, monitoring, reporting | Add screening hooks and suspicious activity event model |
| Technical standards / documentation | Implemented | Architecture doc, local stack doc, scripts, modular code layout | Standards are not enforced by CI or linters yet | Add CI checks, lint, formatting, dependency scanning |
| Technical leadership / clear explanation | Implemented | Architecture and gap documents align code to target responsibilities | Could still use ADR-style decision records | Add ADRs for matching, persistence, and eventing decisions |
| Performance optimization / stability | Partially Implemented | Recovery flow, acceptance scripts, container startup retries | No profiling, no SLOs, no HA testing | Add load tests, profiling, chaos/restart tests |
| 7x24 availability / DR | Planned | Restart recovery in local stack | No HA topology or disaster recovery automation | Add snapshotting, replay, active-standby design artifacts |
| Blockchain / public chain familiarity | Planned | None | No Ethereum/Bitcoin/Solana integration | Add chain adapter interfaces and mock connector services |
| Cloud platform usage | Planned | Docker Compose only | No AWS/Azure/GCP implementation | Add Terraform/Kubernetes manifests and managed-service mapping |
| Same-role exchange depth | Partially Demonstrated | Domain boundaries and architecture reasoning are represented in code and docs | Missing production-grade controls expected in real exchange role | Prioritize auth, ledger, wallet security, compliance, and scaling roadmap |

## Recommended Build Order

To move this prototype closer to the JD, the most efficient implementation order is:

1. API auth, signing, idempotency, rate limiting
2. ledger event model and fee engine
3. withdrawal and wallet control workflows
4. KYC / AML service seams
5. outbox + consumers + replay model
6. service decomposition and deployment topology
