package storage

import (
	"context"

	"senior-architecture/internal/domain"
	"senior-architecture/internal/platform"
)

type NoopStore struct{}

func NewNoopStore() *NoopStore {
	return &NoopStore{}
}

func (s *NoopStore) Bootstrap(context.Context) (*platform.BootstrapState, error) {
	return &platform.BootstrapState{}, nil
}

func (s *NoopStore) PersistAccount(context.Context, *domain.Account) error { return nil }
func (s *NoopStore) PersistOrder(context.Context, *domain.Order) error     { return nil }
func (s *NoopStore) PersistTrades(context.Context, []domain.Trade) error   { return nil }
func (s *NoopStore) PersistAudit(context.Context, []domain.AuditEvent) error {
	return nil
}
func (s *NoopStore) PersistOutbox(context.Context, []domain.OutboxEvent) error { return nil }
func (s *NoopStore) Close() error                                              { return nil }
