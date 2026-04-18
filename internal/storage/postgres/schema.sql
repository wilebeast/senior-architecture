CREATE TABLE IF NOT EXISTS account_balances (
    account_id TEXT NOT NULL,
    asset TEXT NOT NULL,
    available DOUBLE PRECISION NOT NULL,
    locked DOUBLE PRECISION NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, asset)
);

CREATE TABLE IF NOT EXISTS orders (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    side TEXT NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    remaining DOUBLE PRECISION NOT NULL,
    reserved_amount DOUBLE PRECISION NOT NULL,
    sequence BIGINT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orders_symbol_status_sequence
    ON orders (symbol, status, sequence);

CREATE TABLE IF NOT EXISTS trades (
    id TEXT PRIMARY KEY,
    symbol TEXT NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    maker_order_id TEXT NOT NULL,
    taker_order_id TEXT NOT NULL,
    maker_account_id TEXT NOT NULL,
    taker_account_id TEXT NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_trades_symbol_executed_at
    ON trades (symbol, executed_at DESC);

CREATE TABLE IF NOT EXISTS audit_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_audit_events_created_at
    ON audit_events (created_at DESC);
