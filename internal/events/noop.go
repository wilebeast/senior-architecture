package events

import "context"

type NoopBus struct{}

func NewNoopBus() *NoopBus {
	return &NoopBus{}
}

func (b *NoopBus) Publish(context.Context, string, string, any) error { return nil }
func (b *NoopBus) Close() error                                       { return nil }
