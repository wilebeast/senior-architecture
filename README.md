# AtlasX Exchange Core

`AtlasX Exchange Core` is a portfolio-grade project designed to match the responsibilities of a `Senior/Principal Architect - Digital Currency Exchange`.

It is intentionally structured in three layers:

1. `Project design`: one end-to-end exchange platform that covers matching, wallet/ledger, risk, market data, auditability, and compliance hooks.
2. `Technical solution`: architecture decisions, domain boundaries, scaling strategy, security controls, and production evolution path.
3. `Working code`: a runnable Go prototype that demonstrates the critical trading flow.

## 1. Project Design

### Project Goal

Build a digital asset exchange platform for spot trading that supports:

- Low-latency limit order matching
- Wallet and ledger balance management
- Pre-trade risk controls
- Market data dissemination
- Audit-friendly event recording
- Compliance extension points for KYC / AML / suspicious activity checks
- Clear production evolution path toward multi-region, high availability deployment

### Why This Project Matches The JD

This single project covers the core platform domains mentioned in the JD:

- Matching engine
- Wallet system
- Clearing and settlement
- Market data
- API gateway
- KYC / AML integration points
- Security architecture
- High concurrency and scalability planning
- Technical standards, documentation, and operational design

## 2. Technical Solution

The detailed architecture is documented in [docs/architecture.md](/home/star/senior-architecture/docs/architecture.md).

Core technical decisions:

- Language: `Go`
- Prototype style: modular monolith with explicit domain boundaries
- Production target: event-driven microservices with isolated critical paths
- Trading model: price-time priority limit order book
- Consistency model: synchronous pre-trade reservation + deterministic settlement
- Security model: layered defense, segregated hot/cold wallet domains, immutable audit trail
- Local infra stack: `PostgreSQL + Redis + Redpanda + Docker Compose`

## 3. Runnable Prototype

The code now implements a more realistic local exchange core:

- `API server`
- `In-memory trading state with PostgreSQL recovery`
- `Risk engine`
- `Matching engine`
- `Market data service`
- `Audit event store`
- `Redis read cache`
- `Redpanda event publishing`

### Run

Single-process mode without external infra:

```bash
go run ./cmd/exchanged
```

Full local stack:

```bash
docker compose up --build
```

If Docker build cannot reach `proxy.golang.org`, this repository's image build already supports `GOPROXY` override:

```bash
docker compose build --build-arg GOPROXY=https://goproxy.cn,direct
```

Additional local-stack notes are in [docs/local-stack.md](/home/star/senior-architecture/docs/local-stack.md:1).

### Validation Scripts

```bash
chmod +x scripts/*.sh
./scripts/reset-stack.sh
./scripts/smoke-test.sh
./scripts/restart-check.sh
./scripts/inspect-state.sh
./scripts/full-acceptance.sh
```

`full-acceptance.sh` now performs hard assertions for:

- non-zero `executed_at`
- locked balance vs. orderbook consistency
- PostgreSQL / Redis / Redpanda state consistency

### Example Flow

Deposit balances:

```bash
curl -X POST http://localhost:8080/v1/accounts/deposit \
  -H "Content-Type: application/json" \
  -d '{"account_id":"maker-1","asset":"BTC","amount":"2"}'

curl -X POST http://localhost:8080/v1/accounts/deposit \
  -H "Content-Type: application/json" \
  -d '{"account_id":"taker-1","asset":"USDT","amount":"100000"}'
```

Place orders:

```bash
curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"account_id":"maker-1","symbol":"BTC-USDT","side":"sell","price":"65000","quantity":"1"}'

curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"account_id":"taker-1","symbol":"BTC-USDT","side":"buy","price":"66000","quantity":"1"}'
```

Inspect state:

```bash
curl http://localhost:8080/v1/orderbook/BTC-USDT
curl http://localhost:8080/v1/trades/BTC-USDT
curl http://localhost:8080/v1/accounts/maker-1
curl http://localhost:8080/v1/accounts/taker-1
curl http://localhost:8080/v1/audit
```

## Repository Layout

- [cmd/exchanged/main.go](/home/star/senior-architecture/cmd/exchanged/main.go)
- [docs/architecture.md](/home/star/senior-architecture/docs/architecture.md)
- [docs/local-stack.md](/home/star/senior-architecture/docs/local-stack.md)
- [docker-compose.yml](/home/star/senior-architecture/docker-compose.yml)
- [internal/api/server.go](/home/star/senior-architecture/internal/api/server.go)
- [internal/cache/redis.go](/home/star/senior-architecture/internal/cache/redis.go)
- [internal/events/redpanda.go](/home/star/senior-architecture/internal/events/redpanda.go)
- [internal/domain/types.go](/home/star/senior-architecture/internal/domain/types.go)
- [internal/engine/book.go](/home/star/senior-architecture/internal/engine/book.go)
- [internal/marketdata/service.go](/home/star/senior-architecture/internal/marketdata/service.go)
- [internal/platform/exchange.go](/home/star/senior-architecture/internal/platform/exchange.go)
- [internal/risk/service.go](/home/star/senior-architecture/internal/risk/service.go)
- [internal/storage/postgres.go](/home/star/senior-architecture/internal/storage/postgres.go)
- [internal/wallet/service.go](/home/star/senior-architecture/internal/wallet/service.go)

## What This Prototype Demonstrates In Interviews

- How to decompose exchange domains cleanly
- How to keep the matching path deterministic and small
- How wallet reservation and settlement interact with trading
- How to plan security and compliance beyond pure coding
- How to evolve from a prototype into a production-grade Web3 exchange platform
