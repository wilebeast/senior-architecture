package domain

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type Side string

const (
	BuySide  Side = "buy"
	SellSide Side = "sell"
)

type Symbol struct {
	Name       string `json:"name"`
	BaseAsset  string `json:"base_asset"`
	QuoteAsset string `json:"quote_asset"`
}

type OrderStatus string

const (
	OrderStatusOpen            OrderStatus = "open"
	OrderStatusPartiallyFilled OrderStatus = "partially_filled"
	OrderStatusFilled          OrderStatus = "filled"
)

type Order struct {
	ID             string      `json:"id"`
	AccountID      string      `json:"account_id"`
	Symbol         string      `json:"symbol"`
	Side           Side        `json:"side"`
	Price          float64     `json:"price"`
	Quantity       float64     `json:"quantity"`
	Remaining      float64     `json:"remaining"`
	ReservedAmount float64     `json:"reserved_amount"`
	Sequence       int64       `json:"sequence"`
	Status         OrderStatus `json:"status"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type Trade struct {
	ID             string    `json:"id"`
	Symbol         string    `json:"symbol"`
	Price          float64   `json:"price"`
	Quantity       float64   `json:"quantity"`
	MakerOrderID   string    `json:"maker_order_id"`
	TakerOrderID   string    `json:"taker_order_id"`
	MakerAccountID string    `json:"maker_account_id"`
	TakerAccountID string    `json:"taker_account_id"`
	ExecutedAt     time.Time `json:"executed_at"`
}

type OrderRequest struct {
	AccountID string  `json:"account_id"`
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Price     float64 `json:"price,string"`
	Quantity  float64 `json:"quantity,string"`
}

type DepositRequest struct {
	AccountID string  `json:"account_id"`
	Asset     string  `json:"asset"`
	Amount    float64 `json:"amount,string"`
}

type Balance struct {
	Available float64 `json:"available"`
	Locked    float64 `json:"locked"`
}

type Account struct {
	ID        string             `json:"id"`
	Version   int64              `json:"version"`
	UpdatedAt time.Time          `json:"updated_at"`
	Balances  map[string]Balance `json:"balances"`
}

type OutboxEvent struct {
	ID          string         `json:"id"`
	Aggregate   string         `json:"aggregate"`
	AggregateID string         `json:"aggregate_id"`
	Topic       string         `json:"topic"`
	Payload     map[string]any `json:"payload"`
	CreatedAt   time.Time      `json:"created_at"`
}

type AuditEvent struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

var orderSeq int64
var tradeSeq int64
var eventSeq int64

func ParseSide(raw string) (Side, error) {
	switch strings.ToLower(raw) {
	case string(BuySide):
		return BuySide, nil
	case string(SellSide):
		return SellSide, nil
	default:
		return "", fmt.Errorf("unsupported side %q", raw)
	}
}

func NewOrderID() string {
	return fmt.Sprintf("ord-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&orderSeq, 1))
}

func NewTradeID() string {
	return fmt.Sprintf("trd-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&tradeSeq, 1))
}

func NewEventID() string {
	return fmt.Sprintf("evt-%d-%d", time.Now().UnixNano(), atomic.AddInt64(&eventSeq, 1))
}
