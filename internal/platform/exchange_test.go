package platform_test

import (
	"context"
	"testing"
	"time"

	"senior-architecture/internal/cache"
	"senior-architecture/internal/domain"
	"senior-architecture/internal/events"
	"senior-architecture/internal/platform"
	"senior-architecture/internal/storage"
)

func TestPlaceOrderMatchesAndSettles(t *testing.T) {
	exchange, err := platform.NewExchange(context.Background(), storage.NewNoopStore(), cache.NewNoopCache(), events.NewNoopBus())
	if err != nil {
		t.Fatalf("new exchange: %v", err)
	}

	_, err = exchange.Deposit(context.Background(), domain.DepositRequest{
		AccountID: "seller",
		Asset:     "BTC",
		Amount:    2,
	})
	if err != nil {
		t.Fatalf("deposit seller: %v", err)
	}

	_, err = exchange.Deposit(context.Background(), domain.DepositRequest{
		AccountID: "buyer",
		Asset:     "USDT",
		Amount:    100000,
	})
	if err != nil {
		t.Fatalf("deposit buyer: %v", err)
	}

	_, err = exchange.PlaceOrder(context.Background(), domain.OrderRequest{
		AccountID: "seller",
		Symbol:    "BTC-USDT",
		Side:      "sell",
		Price:     65000,
		Quantity:  1,
	})
	if err != nil {
		t.Fatalf("maker order: %v", err)
	}

	result, err := exchange.PlaceOrder(context.Background(), domain.OrderRequest{
		AccountID: "buyer",
		Symbol:    "BTC-USDT",
		Side:      "buy",
		Price:     66000,
		Quantity:  1,
	})
	if err != nil {
		t.Fatalf("taker order: %v", err)
	}

	trades := result["trades"].([]domain.Trade)
	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].ExecutedAt.IsZero() {
		t.Fatalf("expected trade executed_at to be set")
	}

	buyer, err := exchange.GetAccount(context.Background(), "buyer")
	if err != nil {
		t.Fatalf("get buyer: %v", err)
	}
	if buyer.Balances["BTC"].Available != 1 {
		t.Fatalf("expected buyer BTC 1, got %v", buyer.Balances["BTC"].Available)
	}
	if buyer.Balances["USDT"].Available != 35000 {
		t.Fatalf("expected buyer USDT 35000 after refund, got %v", buyer.Balances["USDT"].Available)
	}
	if buyer.Balances["USDT"].Locked != 0 {
		t.Fatalf("expected buyer locked USDT 0, got %v", buyer.Balances["USDT"].Locked)
	}

	seller, err := exchange.GetAccount(context.Background(), "seller")
	if err != nil {
		t.Fatalf("get seller: %v", err)
	}
	if seller.Balances["BTC"].Available != 1 {
		t.Fatalf("expected seller BTC 1 remaining, got %v", seller.Balances["BTC"].Available)
	}
	if seller.Balances["USDT"].Available != 65000 {
		t.Fatalf("expected seller USDT 65000, got %v", seller.Balances["USDT"].Available)
	}
}

func TestBootstrapRebuildsLockedBalancesAndWarmsCache(t *testing.T) {
	store := &bootstrapStore{
		state: &platform.BootstrapState{
			Accounts: []*domain.Account{
				{
					ID: "maker-1",
					Balances: map[string]domain.Balance{
						"BTC":  {Available: 3, Locked: 0},
						"USDT": {Available: 0, Locked: 0},
					},
				},
			},
			Orders: []*domain.Order{
				{
					ID:             "ord-open-1",
					AccountID:      "maker-1",
					Symbol:         "BTC-USDT",
					Side:           domain.SellSide,
					Price:          65000,
					Quantity:       1,
					Remaining:      1,
					ReservedAmount: 1,
					Sequence:       1,
					Status:         domain.OrderStatusOpen,
					CreatedAt:      time.Now().UTC(),
					UpdatedAt:      time.Now().UTC(),
				},
			},
		},
	}
	cacheSpy := newMemoryCache()

	exchange, err := platform.NewExchange(context.Background(), store, cacheSpy, events.NewNoopBus())
	if err != nil {
		t.Fatalf("new exchange: %v", err)
	}

	account, err := exchange.GetAccount(context.Background(), "maker-1")
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if account.Balances["BTC"].Available != 2 {
		t.Fatalf("expected available BTC 2 after rebuild, got %v", account.Balances["BTC"].Available)
	}
	if account.Balances["BTC"].Locked != 1 {
		t.Fatalf("expected locked BTC 1 after rebuild, got %v", account.Balances["BTC"].Locked)
	}

	if len(store.persistedAccounts) == 0 {
		t.Fatalf("expected reconciled accounts to be persisted on bootstrap")
	}
	if _, ok, _ := cacheSpy.GetAccount(context.Background(), "maker-1"); !ok {
		t.Fatalf("expected account cache to be warmed")
	}
	if snapshot, ok, _ := cacheSpy.GetOrderBook(context.Background(), "BTC-USDT"); !ok || len(snapshot["asks"].([]domain.Order)) != 1 {
		t.Fatalf("expected orderbook cache to be warmed with restored order")
	}
}

type bootstrapStore struct {
	state             *platform.BootstrapState
	persistedAccounts []*domain.Account
}

func (s *bootstrapStore) Bootstrap(context.Context) (*platform.BootstrapState, error) {
	return s.state, nil
}
func (s *bootstrapStore) PersistAccount(_ context.Context, account *domain.Account) error {
	cloned := &domain.Account{ID: account.ID, Balances: map[string]domain.Balance{}}
	for asset, balance := range account.Balances {
		cloned.Balances[asset] = balance
	}
	s.persistedAccounts = append(s.persistedAccounts, cloned)
	return nil
}
func (s *bootstrapStore) PersistOrder(context.Context, *domain.Order) error       { return nil }
func (s *bootstrapStore) PersistTrades(context.Context, []domain.Trade) error     { return nil }
func (s *bootstrapStore) PersistAudit(context.Context, []domain.AuditEvent) error { return nil }
func (s *bootstrapStore) Close() error                                            { return nil }

type memoryCache struct {
	accounts  map[string]*domain.Account
	orderbook map[string]map[string]any
	trades    map[string][]domain.Trade
}

func newMemoryCache() *memoryCache {
	return &memoryCache{
		accounts:  map[string]*domain.Account{},
		orderbook: map[string]map[string]any{},
		trades:    map[string][]domain.Trade{},
	}
}

func (c *memoryCache) GetAccount(_ context.Context, accountID string) (*domain.Account, bool, error) {
	account, ok := c.accounts[accountID]
	return account, ok, nil
}
func (c *memoryCache) SetAccount(_ context.Context, account *domain.Account) error {
	cloned := &domain.Account{ID: account.ID, Balances: map[string]domain.Balance{}}
	for asset, balance := range account.Balances {
		cloned.Balances[asset] = balance
	}
	c.accounts[account.ID] = cloned
	return nil
}
func (c *memoryCache) GetOrderBook(_ context.Context, symbol string) (map[string]any, bool, error) {
	snapshot, ok := c.orderbook[symbol]
	return snapshot, ok, nil
}
func (c *memoryCache) SetOrderBook(_ context.Context, symbol string, snapshot map[string]any) error {
	c.orderbook[symbol] = snapshot
	return nil
}
func (c *memoryCache) GetTrades(_ context.Context, symbol string) ([]domain.Trade, bool, error) {
	trades, ok := c.trades[symbol]
	return trades, ok, nil
}
func (c *memoryCache) SetTrades(_ context.Context, symbol string, trades []domain.Trade) error {
	c.trades[symbol] = trades
	return nil
}
func (c *memoryCache) Close() error { return nil }
