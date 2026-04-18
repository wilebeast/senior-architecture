package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"senior-architecture/internal/api"
	"senior-architecture/internal/cache"
	"senior-architecture/internal/config"
	"senior-architecture/internal/events"
	"senior-architecture/internal/platform"
	"senior-architecture/internal/storage"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	var store platform.Store = storage.NewNoopStore()
	if cfg.PostgresDSN != "" {
		postgresStore, err := withRetry(ctx, "postgres", func() (*storage.PostgresStore, error) {
			return storage.NewPostgresStore(ctx, cfg.PostgresDSN)
		})
		if err != nil {
			log.Fatal(err)
		}
		store = postgresStore
	}

	var cacheClient platform.Cache = cache.NewNoopCache()
	if cfg.RedisAddr != "" {
		redisCache, err := withRetry(ctx, "redis", func() (*cache.RedisCache, error) {
			return cache.NewRedisCache(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.AccountCacheTTL, cfg.OrderBookTTL, cfg.TradesCacheTTL)
		})
		if err != nil {
			log.Fatal(err)
		}
		cacheClient = redisCache
	}

	var bus platform.EventBus = events.NewNoopBus()
	if len(cfg.RedpandaBrokers) > 0 {
		redpandaBus, err := withRetry(ctx, "redpanda", func() (*events.RedpandaBus, error) {
			return events.NewRedpandaBus(cfg.RedpandaBrokers, []string{platform.TopicOrders, platform.TopicTrades, platform.TopicAudit})
		})
		if err != nil {
			log.Fatal(err)
		}
		bus = redpandaBus
	}

	exchange, err := platform.NewExchange(ctx, store, cacheClient, bus)
	if err != nil {
		log.Fatal(err)
	}
	defer exchange.Close()

	server := api.NewServer(exchange)

	log.Printf("atlasx exchange core listening on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func withRetry[T any](ctx context.Context, name string, fn func() (T, error)) (T, error) {
	var zero T
	const maxAttempts = 30

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		value, err := fn()
		if err == nil {
			if attempt > 1 {
				log.Printf("%s became available after %d attempts", name, attempt)
			}
			return value, nil
		}

		if attempt == maxAttempts {
			return zero, fmt.Errorf("%s unavailable after %d attempts: %w", name, maxAttempts, err)
		}

		log.Printf("%s unavailable on attempt %d/%d: %v", name, attempt, maxAttempts, err)

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return zero, fmt.Errorf("%s unavailable", name)
}
