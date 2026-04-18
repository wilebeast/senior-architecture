package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"senior-architecture/internal/domain"
)

type RedisCache struct {
	client       *redis.Client
	accountTTL   time.Duration
	orderBookTTL time.Duration
	tradesTTL    time.Duration
}

func NewRedisCache(addr, password string, db int, accountTTL, orderBookTTL, tradesTTL time.Duration) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &RedisCache{
		client:       client,
		accountTTL:   accountTTL,
		orderBookTTL: orderBookTTL,
		tradesTTL:    tradesTTL,
	}, nil
}

func (c *RedisCache) GetAccount(ctx context.Context, accountID string) (*domain.Account, bool, error) {
	var account domain.Account
	ok, err := c.getJSON(ctx, "account:"+accountID, &account)
	if err != nil || !ok {
		return nil, ok, err
	}
	return &account, true, nil
}

func (c *RedisCache) SetAccount(ctx context.Context, account *domain.Account) error {
	return c.setJSON(ctx, "account:"+account.ID, account, c.accountTTL)
}

func (c *RedisCache) GetOrderBook(ctx context.Context, symbol string) (map[string]any, bool, error) {
	var snapshot map[string]any
	ok, err := c.getJSON(ctx, "orderbook:"+symbol, &snapshot)
	return snapshot, ok, err
}

func (c *RedisCache) SetOrderBook(ctx context.Context, symbol string, snapshot map[string]any) error {
	return c.setJSON(ctx, "orderbook:"+symbol, snapshot, c.orderBookTTL)
}

func (c *RedisCache) GetTrades(ctx context.Context, symbol string) ([]domain.Trade, bool, error) {
	var trades []domain.Trade
	ok, err := c.getJSON(ctx, "trades:"+symbol, &trades)
	return trades, ok, err
}

func (c *RedisCache) SetTrades(ctx context.Context, symbol string, trades []domain.Trade) error {
	return c.setJSON(ctx, "trades:"+symbol, trades, c.tradesTTL)
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) getJSON(ctx context.Context, key string, dest any) (bool, error) {
	value, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(value), dest)
}

func (c *RedisCache) setJSON(ctx context.Context, key string, payload any, ttl time.Duration) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, data, ttl).Err()
}
