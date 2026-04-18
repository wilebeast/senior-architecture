package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	PostgresDSN     string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	RedpandaBrokers []string
	AccountCacheTTL time.Duration
	OrderBookTTL    time.Duration
	TradesCacheTTL  time.Duration
}

func Load() Config {
	return Config{
		HTTPAddr:        getEnv("HTTP_ADDR", ":8080"),
		PostgresDSN:     os.Getenv("POSTGRES_DSN"),
		RedisAddr:       os.Getenv("REDIS_ADDR"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		RedisDB:         0,
		RedpandaBrokers: splitCSV(os.Getenv("REDPANDA_BROKERS")),
		AccountCacheTTL: 5 * time.Minute,
		OrderBookTTL:    2 * time.Minute,
		TradesCacheTTL:  2 * time.Minute,
	}
}

func splitCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
