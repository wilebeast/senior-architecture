# AtlasX Exchange Core

`AtlasX Exchange Core` is a portfolio-grade exchange core for a `Senior/Principal Architect - Digital Currency Exchange` role.

It is not a full production exchange. It is a `stateful prototype` that demonstrates:

- matching and settlement flow
- balance reservation and recovery
- PostgreSQL persistence
- Redis read cache
- Redpanda event publishing
- local operational validation with Docker and acceptance scripts

## Status

This repository follows the same status model as [docs/architecture.md](/home/star/senior-architecture/docs/architecture.md:1):

- `Implemented`
- `Partially Implemented`
- `Planned`

### Implemented

- spot order placement API
- price-time-priority in-memory matching
- pre-trade reservation and deterministic settlement
- PostgreSQL persistence for balances, orders, trades, and audit events
- Redis cache for account, orderbook, and trades
- Redpanda event publishing for `exchange.orders`, `exchange.trades`, `exchange.audit`
- restart recovery for balances and open orders
- smoke, restart, inspect, and acceptance scripts

### Partially Implemented

- API gateway behavior
  HTTP endpoints exist, but there is no auth, signing, idempotency, or rate limiting.
- wallet / ledger
  Reservation and settlement exist, but there is no blockchain deposit or withdrawal workflow.
- market data
  Orderbook snapshots and trade history exist, but no streaming or candles.
- recovery architecture
  Bootstrap recovery exists, but not snapshot + replay.

### Planned

- KYC / AML workflows
- hot / warm / cold wallet controls
- HSM / KMS / MPC integration
- fee engine
- cloud-native deployment stack
- multi-process matching partitions
- production security controls

## Documents

- [Architecture Status](/home/star/senior-architecture/docs/architecture.md:1)
- [Architecture Gap Matrix](/home/star/senior-architecture/docs/architecture-gap-matrix.md:1)
- [Local Stack](/home/star/senior-architecture/docs/local-stack.md:1)

## Local Run

Single-process mode:

```bash
go run ./cmd/exchanged
```

Full local stack:

```bash
docker compose up --build
```

If Docker build cannot reach `proxy.golang.org`, rebuild with:

```bash
docker compose build --build-arg GOPROXY=https://goproxy.cn,direct
```

## Validation

```bash
chmod +x scripts/*.sh
./scripts/reset-stack.sh
./scripts/smoke-test.sh
./scripts/debug-flow.sh
./scripts/restart-check.sh
./scripts/inspect-state.sh
./scripts/full-acceptance.sh
```

`full-acceptance.sh` asserts:

- non-zero `executed_at`
- locked balance vs. orderbook consistency
- PostgreSQL / Redis / Redpanda consistency

## Example Flow

```bash
curl -X POST http://localhost:8080/v1/accounts/deposit \
  -H "Content-Type: application/json" \
  -d '{"account_id":"maker-1","asset":"BTC","amount":"2"}'

curl -X POST http://localhost:8080/v1/accounts/deposit \
  -H "Content-Type: application/json" \
  -d '{"account_id":"taker-1","asset":"USDT","amount":"100000"}'

curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"account_id":"maker-1","symbol":"BTC-USDT","side":"sell","price":"65000","quantity":"1"}'

curl -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"account_id":"taker-1","symbol":"BTC-USDT","side":"buy","price":"66000","quantity":"1"}'
```

Inspect:

```bash
curl http://localhost:8080/v1/orderbook/BTC-USDT
curl http://localhost:8080/v1/trades/BTC-USDT
curl http://localhost:8080/v1/accounts/maker-1
curl http://localhost:8080/v1/accounts/taker-1
curl http://localhost:8080/v1/audit
```

## Repo Layout

- [cmd/exchanged/main.go](/home/star/senior-architecture/cmd/exchanged/main.go)
- [internal/api/server.go](/home/star/senior-architecture/internal/api/server.go)
- [internal/engine/book.go](/home/star/senior-architecture/internal/engine/book.go)
- [internal/platform/exchange.go](/home/star/senior-architecture/internal/platform/exchange.go)
- [internal/storage/postgres.go](/home/star/senior-architecture/internal/storage/postgres.go)
- [internal/cache/redis.go](/home/star/senior-architecture/internal/cache/redis.go)
- [internal/events/redpanda.go](/home/star/senior-architecture/internal/events/redpanda.go)
- [scripts/full-acceptance.sh](/home/star/senior-architecture/scripts/full-acceptance.sh)

## Interview Value

This repository is useful in interviews because it shows:

- how to structure exchange domains
- how to keep the matching path small and deterministic
- how to reason about persistence, caching, and event publication
- how to distinguish current implementation from production target architecture
