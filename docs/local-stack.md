# Local Stack

This project now supports a local stateful stack with:

- `PostgreSQL` for balances, orders, trades, and audit persistence
- `Redis` for account, order book, and trades read cache
- `Redpanda` for order, trade, and audit event topics
- `Docker Compose` for one-command local bootstrap

## Start

```bash
docker compose up --build
```

If the image build times out while downloading Go modules, rebuild with a different Go proxy:

```bash
docker compose build --build-arg GOPROXY=https://goproxy.cn,direct
docker compose up
```

## Config

Environment variables used by the application:

- `HTTP_ADDR`
- `POSTGRES_DSN`
- `REDIS_ADDR`
- `REDIS_PASSWORD`
- `REDPANDA_BROKERS`

Default compose wiring:

- HTTP: `localhost:8080`
- PostgreSQL: `localhost:5432`
- Redis: `localhost:6379`
- Redpanda: `localhost:19092`

## Persistence Model

### PostgreSQL

The service auto-applies schema on startup and persists:

- `account_balances`
- `orders`
- `trades`
- `audit_events`

On restart, the service rebuilds runtime state from:

- account balances
- open and partially filled orders
- recent trades
- recent audit events

### Redis

Redis is used as a read-through / write-through cache for:

- account snapshots
- order book snapshots
- trade lists

### Redpanda Topics

The service publishes JSON events to:

- `exchange.orders`
- `exchange.trades`
- `exchange.audit`
