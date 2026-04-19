package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	_ "github.com/lib/pq"

	"senior-architecture/internal/domain"
	"senior-architecture/internal/platform"
)

//go:embed postgres/schema.sql
var postgresSchema string

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if _, err := db.ExecContext(ctx, postgresSchema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Bootstrap(ctx context.Context) (*platform.BootstrapState, error) {
	accounts, err := s.loadAccounts(ctx)
	if err != nil {
		return nil, err
	}
	orders, err := s.loadOpenOrders(ctx)
	if err != nil {
		return nil, err
	}
	trades, err := s.loadTrades(ctx)
	if err != nil {
		return nil, err
	}
	audit, err := s.loadAudit(ctx)
	if err != nil {
		return nil, err
	}

	return &platform.BootstrapState{
		Accounts: accounts,
		Orders:   orders,
		Trades:   trades,
		Audit:    audit,
	}, nil
}

func (s *PostgresStore) PersistAccount(ctx context.Context, account *domain.Account) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	version, err := lockAndBumpAccountVersion(ctx, tx, account)
	if err != nil {
		return err
	}

	for asset, balance := range account.Balances {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO account_balances (account_id, asset, available, locked, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (account_id, asset) DO UPDATE
			SET available = EXCLUDED.available,
			    locked = EXCLUDED.locked,
			    updated_at = NOW()
		`, account.ID, asset, balance.Available, balance.Locked); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	account.Version = version
	account.UpdatedAt = time.Now().UTC()
	return nil
}

func (s *PostgresStore) PersistOrder(ctx context.Context, order *domain.Order) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO orders (
			id, account_id, symbol, side, price, quantity, remaining,
			reserved_amount, sequence, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (id) DO UPDATE
		SET remaining = EXCLUDED.remaining,
		    reserved_amount = EXCLUDED.reserved_amount,
		    sequence = EXCLUDED.sequence,
		    status = EXCLUDED.status,
		    updated_at = EXCLUDED.updated_at
	`, order.ID, order.AccountID, order.Symbol, string(order.Side), order.Price, order.Quantity, order.Remaining, order.ReservedAmount, order.Sequence, string(order.Status), order.CreatedAt, order.UpdatedAt); err != nil {
		return err
	}

	payload, err := toPayload(order)
	if err != nil {
		return err
	}
	if err := persistOutboxEvents(ctx, tx, []domain.OutboxEvent{{
		ID:          domain.NewEventID(),
		Aggregate:   "order",
		AggregateID: order.ID,
		Topic:       platform.TopicOrders,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
	}}); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *PostgresStore) PersistTrades(ctx context.Context, trades []domain.Trade) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var outbox []domain.OutboxEvent
	for _, trade := range trades {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO trades (
				id, symbol, price, quantity, maker_order_id, taker_order_id,
				maker_account_id, taker_account_id, executed_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING
		`, trade.ID, trade.Symbol, trade.Price, trade.Quantity, trade.MakerOrderID, trade.TakerOrderID, trade.MakerAccountID, trade.TakerAccountID, trade.ExecutedAt); err != nil {
			return err
		}
		payload, err := toPayload(trade)
		if err != nil {
			return err
		}
		outbox = append(outbox, domain.OutboxEvent{
			ID:          domain.NewEventID(),
			Aggregate:   "trade",
			AggregateID: trade.ID,
			Topic:       platform.TopicTrades,
			Payload:     payload,
			CreatedAt:   time.Now().UTC(),
		})
	}
	if err := persistOutboxEvents(ctx, tx, outbox); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) PersistAudit(ctx context.Context, events []domain.AuditEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var outbox []domain.OutboxEvent
	for _, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO audit_events (id, event_type, payload, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO NOTHING
		`, event.ID, event.Type, payload, event.CreatedAt); err != nil {
			return err
		}
		outbox = append(outbox, domain.OutboxEvent{
			ID:          domain.NewEventID(),
			Aggregate:   "audit",
			AggregateID: event.ID,
			Topic:       platform.TopicAudit,
			Payload:     event.Payload,
			CreatedAt:   time.Now().UTC(),
		})
	}
	if err := persistOutboxEvents(ctx, tx, outbox); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) PersistOutbox(ctx context.Context, events []domain.OutboxEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := persistOutboxEvents(ctx, tx, events); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) loadAccounts(ctx context.Context) ([]*domain.Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.version, a.updated_at, b.asset, b.available, b.locked
		FROM accounts a
		JOIN account_balances b ON b.account_id = a.id
		ORDER BY a.id, b.asset
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := map[string]*domain.Account{}
	for rows.Next() {
		var accountID, asset string
		var version int64
		var updatedAt time.Time
		var available, locked float64
		if err := rows.Scan(&accountID, &version, &updatedAt, &asset, &available, &locked); err != nil {
			return nil, err
		}
		account, ok := accounts[accountID]
		if !ok {
			account = &domain.Account{ID: accountID, Version: version, UpdatedAt: updatedAt, Balances: map[string]domain.Balance{}}
			accounts[accountID] = account
		}
		account.Balances[asset] = domain.Balance{Available: available, Locked: locked}
	}

	keys := make([]string, 0, len(accounts))
	for key := range accounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]*domain.Account, 0, len(keys))
	for _, key := range keys {
		out = append(out, accounts[key])
	}
	return out, rows.Err()
}

func (s *PostgresStore) loadOpenOrders(ctx context.Context) ([]*domain.Order, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, symbol, side, price, quantity, remaining,
		       reserved_amount, sequence, status, created_at, updated_at
		FROM orders
		WHERE status IN ('open', 'partially_filled')
		ORDER BY symbol, sequence
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Order
	for rows.Next() {
		var order domain.Order
		var side, status string
		if err := rows.Scan(&order.ID, &order.AccountID, &order.Symbol, &side, &order.Price, &order.Quantity, &order.Remaining, &order.ReservedAmount, &order.Sequence, &status, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, err
		}
		order.Side = domain.Side(side)
		order.Status = domain.OrderStatus(status)
		out = append(out, &order)
	}
	return out, rows.Err()
}

func (s *PostgresStore) loadTrades(ctx context.Context) ([]domain.Trade, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, symbol, price, quantity, maker_order_id, taker_order_id,
		       maker_account_id, taker_account_id, executed_at
		FROM trades
		ORDER BY executed_at, id
		LIMIT 1000
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Trade
	for rows.Next() {
		var trade domain.Trade
		if err := rows.Scan(&trade.ID, &trade.Symbol, &trade.Price, &trade.Quantity, &trade.MakerOrderID, &trade.TakerOrderID, &trade.MakerAccountID, &trade.TakerAccountID, &trade.ExecutedAt); err != nil {
			return nil, err
		}
		out = append(out, trade)
	}
	return out, rows.Err()
}

func (s *PostgresStore) loadAudit(ctx context.Context) ([]domain.AuditEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, event_type, payload, created_at
		FROM audit_events
		ORDER BY created_at, id
		LIMIT 1000
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.AuditEvent
	for rows.Next() {
		var event domain.AuditEvent
		var payload []byte
		if err := rows.Scan(&event.ID, &event.Type, &payload, &event.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &event.Payload); err != nil {
			return nil, err
		}
		out = append(out, event)
	}
	return out, rows.Err()
}

func lockAndBumpAccountVersion(ctx context.Context, tx *sql.Tx, account *domain.Account) (int64, error) {
	var currentVersion int64
	err := tx.QueryRowContext(ctx, `SELECT version FROM accounts WHERE id = $1 FOR UPDATE`, account.ID).Scan(&currentVersion)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if account.Version != 0 {
			return 0, fmt.Errorf("account %s version conflict: expected new account, got version %d", account.ID, account.Version)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO accounts (id, version, updated_at)
			VALUES ($1, $2, NOW())
		`, account.ID, 1); err != nil {
			return 0, err
		}
		return 1, nil
	case err != nil:
		return 0, err
	}

	if currentVersion != account.Version {
		return 0, fmt.Errorf("account %s version conflict: expected %d got %d", account.ID, account.Version, currentVersion)
	}

	nextVersion := currentVersion + 1
	if _, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET version = $2, updated_at = NOW()
		WHERE id = $1 AND version = $3
	`, account.ID, nextVersion, currentVersion); err != nil {
		return 0, err
	}
	return nextVersion, nil
}

func persistOutboxEvents(ctx context.Context, tx *sql.Tx, events []domain.OutboxEvent) error {
	for _, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO outbox_events (id, aggregate_type, aggregate_id, topic, payload, created_at, published_at)
			VALUES ($1, $2, $3, $4, $5, $6, NULL)
			ON CONFLICT (id) DO NOTHING
		`, event.ID, event.Aggregate, event.AggregateID, event.Topic, payload, event.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func toPayload(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}
