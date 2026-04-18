package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

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

	return tx.Commit()
}

func (s *PostgresStore) PersistOrder(ctx context.Context, order *domain.Order) error {
	_, err := s.db.ExecContext(ctx, `
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
	`, order.ID, order.AccountID, order.Symbol, string(order.Side), order.Price, order.Quantity, order.Remaining, order.ReservedAmount, order.Sequence, string(order.Status), order.CreatedAt, order.UpdatedAt)
	return err
}

func (s *PostgresStore) PersistTrades(ctx context.Context, trades []domain.Trade) error {
	for _, trade := range trades {
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO trades (
				id, symbol, price, quantity, maker_order_id, taker_order_id,
				maker_account_id, taker_account_id, executed_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO NOTHING
		`, trade.ID, trade.Symbol, trade.Price, trade.Quantity, trade.MakerOrderID, trade.TakerOrderID, trade.MakerAccountID, trade.TakerAccountID, trade.ExecutedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) PersistAudit(ctx context.Context, events []domain.AuditEvent) error {
	for _, event := range events {
		payload, err := json.Marshal(event.Payload)
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO audit_events (id, event_type, payload, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO NOTHING
		`, event.ID, event.Type, payload, event.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *PostgresStore) Close() error {
	return s.db.Close()
}

func (s *PostgresStore) loadAccounts(ctx context.Context) ([]*domain.Account, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_id, asset, available, locked
		FROM account_balances
		ORDER BY account_id, asset
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	accounts := map[string]*domain.Account{}
	for rows.Next() {
		var accountID, asset string
		var available, locked float64
		if err := rows.Scan(&accountID, &asset, &available, &locked); err != nil {
			return nil, err
		}
		account, ok := accounts[accountID]
		if !ok {
			account = &domain.Account{ID: accountID, Balances: map[string]domain.Balance{}}
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
