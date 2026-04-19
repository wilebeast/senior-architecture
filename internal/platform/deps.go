package platform

import (
	"context"

	"senior-architecture/internal/domain"
)

const (
	TopicOrders = "exchange.orders"
	TopicTrades = "exchange.trades"
	TopicAudit  = "exchange.audit"
)

type BootstrapState struct {
	Accounts []*domain.Account
	Orders   []*domain.Order
	Trades   []domain.Trade
	Audit    []domain.AuditEvent
}

type Store interface {
	Bootstrap(ctx context.Context) (*BootstrapState, error)
	PersistAccount(ctx context.Context, account *domain.Account) error
	PersistOrder(ctx context.Context, order *domain.Order) error
	PersistTrades(ctx context.Context, trades []domain.Trade) error
	PersistAudit(ctx context.Context, events []domain.AuditEvent) error
	PersistOutbox(ctx context.Context, events []domain.OutboxEvent) error
	Close() error
}

type Cache interface {
	GetAccount(ctx context.Context, accountID string) (*domain.Account, bool, error)
	SetAccount(ctx context.Context, account *domain.Account) error
	GetOrderBook(ctx context.Context, symbol string) (map[string]any, bool, error)
	SetOrderBook(ctx context.Context, symbol string, snapshot map[string]any) error
	GetTrades(ctx context.Context, symbol string) ([]domain.Trade, bool, error)
	SetTrades(ctx context.Context, symbol string, trades []domain.Trade) error
	Close() error
}

type EventBus interface {
	Publish(ctx context.Context, topic, key string, payload any) error
	Close() error
}
