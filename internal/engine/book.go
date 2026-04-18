package engine

import (
	"sort"
	"sync"
	"time"

	"senior-architecture/internal/domain"
)

type MatchResult struct {
	Accepted *domain.Order  `json:"accepted"`
	Fills    []Fill         `json:"-"`
	Trades   []domain.Trade `json:"trades"`
}

type Fill struct {
	Maker *domain.Order
	Trade domain.Trade
}

type OrderBook struct {
	Symbol string

	mu   sync.Mutex
	bids []*domain.Order
	asks []*domain.Order
	seq  int64
}

func NewOrderBook(symbol string) *OrderBook {
	return &OrderBook{Symbol: symbol}
}

func (b *OrderBook) Restore(order *domain.Order) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if order.Sequence > b.seq {
		b.seq = order.Sequence
	}

	switch order.Side {
	case domain.BuySide:
		b.bids = append(b.bids, order)
	case domain.SellSide:
		b.asks = append(b.asks, order)
	}

	b.sortBooks()
}

func (b *OrderBook) Submit(order *domain.Order) MatchResult {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.seq++
	order.Sequence = b.seq
	order.Remaining = order.Quantity

	var fills []Fill
	switch order.Side {
	case domain.BuySide:
		fills = b.matchBuy(order)
		if order.Remaining > 0 {
			b.bids = append(b.bids, order)
			b.sortBooks()
		}
	case domain.SellSide:
		fills = b.matchSell(order)
		if order.Remaining > 0 {
			b.asks = append(b.asks, order)
			b.sortBooks()
		}
	}

	trades := make([]domain.Trade, 0, len(fills))
	for _, fill := range fills {
		trades = append(trades, fill.Trade)
	}

	return MatchResult{
		Accepted: order,
		Fills:    fills,
		Trades:   trades,
	}
}

func (b *OrderBook) Snapshot() map[string]any {
	b.mu.Lock()
	defer b.mu.Unlock()

	return map[string]any{
		"symbol": b.Symbol,
		"bids":   cloneOrders(b.bids),
		"asks":   cloneOrders(b.asks),
	}
}

func (b *OrderBook) matchBuy(order *domain.Order) []Fill {
	var fills []Fill

	for len(b.asks) > 0 && order.Remaining > 0 {
		bestAsk := b.asks[0]
		if bestAsk.Price > order.Price {
			break
		}

		tradeQty := min(order.Remaining, bestAsk.Remaining)
		bestAsk.Remaining -= tradeQty
		order.Remaining -= tradeQty

		fill := Fill{
			Maker: bestAsk,
			Trade: domain.Trade{
				ID:             domain.NewTradeID(),
				Symbol:         order.Symbol,
				Price:          bestAsk.Price,
				Quantity:       tradeQty,
				MakerOrderID:   bestAsk.ID,
				TakerOrderID:   order.ID,
				MakerAccountID: bestAsk.AccountID,
				TakerAccountID: order.AccountID,
				ExecutedAt:     time.Now().UTC(),
			},
		}
		fills = append(fills, fill)

		if bestAsk.Remaining == 0 {
			b.asks = b.asks[1:]
		}
	}

	return fills
}

func (b *OrderBook) matchSell(order *domain.Order) []Fill {
	var fills []Fill

	for len(b.bids) > 0 && order.Remaining > 0 {
		bestBid := b.bids[0]
		if bestBid.Price < order.Price {
			break
		}

		tradeQty := min(order.Remaining, bestBid.Remaining)
		bestBid.Remaining -= tradeQty
		order.Remaining -= tradeQty

		fill := Fill{
			Maker: bestBid,
			Trade: domain.Trade{
				ID:             domain.NewTradeID(),
				Symbol:         order.Symbol,
				Price:          bestBid.Price,
				Quantity:       tradeQty,
				MakerOrderID:   bestBid.ID,
				TakerOrderID:   order.ID,
				MakerAccountID: bestBid.AccountID,
				TakerAccountID: order.AccountID,
				ExecutedAt:     time.Now().UTC(),
			},
		}
		fills = append(fills, fill)

		if bestBid.Remaining == 0 {
			b.bids = b.bids[1:]
		}
	}

	return fills
}

func (b *OrderBook) sortBooks() {
	sort.SliceStable(b.bids, func(i, j int) bool {
		if b.bids[i].Price == b.bids[j].Price {
			return b.bids[i].Sequence < b.bids[j].Sequence
		}
		return b.bids[i].Price > b.bids[j].Price
	})

	sort.SliceStable(b.asks, func(i, j int) bool {
		if b.asks[i].Price == b.asks[j].Price {
			return b.asks[i].Sequence < b.asks[j].Sequence
		}
		return b.asks[i].Price < b.asks[j].Price
	})
}

func cloneOrders(input []*domain.Order) []domain.Order {
	out := make([]domain.Order, 0, len(input))
	for _, order := range input {
		out = append(out, *order)
	}
	return out
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
