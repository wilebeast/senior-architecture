package cache

import (
	"context"

	"senior-architecture/internal/domain"
)

type NoopCache struct{}

func NewNoopCache() *NoopCache {
	return &NoopCache{}
}

func (c *NoopCache) GetAccount(context.Context, string) (*domain.Account, bool, error) {
	return nil, false, nil
}

func (c *NoopCache) SetAccount(context.Context, *domain.Account) error { return nil }

func (c *NoopCache) GetOrderBook(context.Context, string) (map[string]any, bool, error) {
	return nil, false, nil
}

func (c *NoopCache) SetOrderBook(context.Context, string, map[string]any) error { return nil }

func (c *NoopCache) GetTrades(context.Context, string) ([]domain.Trade, bool, error) {
	return nil, false, nil
}

func (c *NoopCache) SetTrades(context.Context, string, []domain.Trade) error { return nil }

func (c *NoopCache) Close() error { return nil }
