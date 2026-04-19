package platform

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"senior-architecture/internal/domain"
	"senior-architecture/internal/engine"
	"senior-architecture/internal/marketdata"
	"senior-architecture/internal/risk"
	"senior-architecture/internal/wallet"
)

type Exchange struct {
	symbols    map[string]domain.Symbol
	books      map[string]*engine.OrderBook
	wallet     *wallet.Service
	risk       *risk.Service
	marketData *marketdata.Service
	store      Store
	cache      Cache
	bus        EventBus

	auditMu sync.RWMutex
	audit   []domain.AuditEvent
}

func NewExchange(ctx context.Context, store Store, cache Cache, bus EventBus) (*Exchange, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	if cache == nil {
		return nil, fmt.Errorf("cache is required")
	}
	if bus == nil {
		return nil, fmt.Errorf("event bus is required")
	}

	symbols := map[string]domain.Symbol{
		"BTC-USDT": {Name: "BTC-USDT", BaseAsset: "BTC", QuoteAsset: "USDT"},
		"ETH-USDT": {Name: "ETH-USDT", BaseAsset: "ETH", QuoteAsset: "USDT"},
	}

	books := make(map[string]*engine.OrderBook, len(symbols))
	for name := range symbols {
		books[name] = engine.NewOrderBook(name)
	}

	state, err := store.Bootstrap(ctx)
	if err != nil {
		return nil, fmt.Errorf("bootstrap exchange state: %w", err)
	}

	walletService := wallet.NewService(state.Accounts...)
	marketDataService := marketdata.NewService()
	marketDataService.Load(state.Trades)

	exchange := &Exchange{
		symbols:    symbols,
		books:      books,
		wallet:     walletService,
		risk:       risk.NewService(walletService),
		marketData: marketDataService,
		store:      store,
		cache:      cache,
		bus:        bus,
		audit:      append([]domain.AuditEvent(nil), state.Audit...),
	}

	for _, order := range state.Orders {
		book, ok := exchange.books[order.Symbol]
		if !ok {
			continue
		}
		book.Restore(order)
	}

	if err := exchange.rebuildRecoveredState(ctx, state.Accounts, state.Orders); err != nil {
		return nil, err
	}

	return exchange, nil
}

func (e *Exchange) Close() error {
	var firstErr error
	if err := e.bus.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := e.cache.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := e.store.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (e *Exchange) Deposit(ctx context.Context, req domain.DepositRequest) (*domain.Account, error) {
	account := e.wallet.GetAccount(req.AccountID)
	if err := applyDeposit(account, req.Asset, req.Amount); err != nil {
		return nil, err
	}

	if err := e.store.PersistAccount(ctx, account); err != nil {
		return nil, err
	}
	e.wallet.ReplaceAccount(account)
	e.syncAccountCache(ctx, account)

	event := domain.AuditEvent{
		ID:        domain.NewEventID(),
		Type:      "deposit.completed",
		Payload:   map[string]any{"account_id": req.AccountID, "asset": req.Asset, "amount": req.Amount, "account_version": account.Version},
		CreatedAt: time.Now().UTC(),
	}
	if err := e.recordAudit(ctx, event); err != nil {
		return nil, err
	}

	return account, nil
}

func applyDeposit(account *domain.Account, asset string, amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if account.Balances == nil {
		account.Balances = make(map[string]domain.Balance)
	}
	balance := account.Balances[asset]
	balance.Available += amount
	account.Balances[asset] = balance
	return nil
}

func (e *Exchange) PlaceOrder(ctx context.Context, req domain.OrderRequest) (map[string]any, error) {
	symbol, ok := e.symbols[req.Symbol]
	if !ok {
		return nil, fmt.Errorf("unsupported symbol %s", req.Symbol)
	}

	side, err := domain.ParseSide(req.Side)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	order := &domain.Order{
		ID:        domain.NewOrderID(),
		AccountID: req.AccountID,
		Symbol:    req.Symbol,
		Side:      side,
		Price:     req.Price,
		Quantity:  req.Quantity,
		Status:    domain.OrderStatusOpen,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := e.risk.ValidateOrder(symbol, order); err != nil {
		return nil, err
	}
	if err := e.wallet.Reserve(symbol, order); err != nil {
		return nil, err
	}
	if err := e.syncAccount(ctx, e.wallet.GetAccount(order.AccountID)); err != nil {
		return nil, err
	}

	result := e.books[req.Symbol].Submit(order)
	e.updateOrderStatus(result.Accepted)

	changedOrders := map[string]*domain.Order{result.Accepted.ID: result.Accepted}
	changedAccounts := map[string]*domain.Account{
		order.AccountID: e.wallet.GetAccount(order.AccountID),
	}

	for _, fill := range result.Fills {
		if err := e.applyTrade(fill, symbol, result.Accepted); err != nil {
			return nil, err
		}
		changedOrders[fill.Maker.ID] = fill.Maker
		changedAccounts[fill.Maker.AccountID] = e.wallet.GetAccount(fill.Maker.AccountID)
		changedAccounts[order.AccountID] = e.wallet.GetAccount(order.AccountID)
	}

	if result.Accepted.Remaining == 0 {
		if err := e.wallet.ReleaseRemaining(symbol, result.Accepted); err != nil {
			return nil, err
		}
		changedAccounts[order.AccountID] = e.wallet.GetAccount(order.AccountID)
	}

	if err := e.persistChangedAccounts(ctx, changedAccounts); err != nil {
		return nil, err
	}
	if err := e.persistChangedOrders(ctx, changedOrders); err != nil {
		return nil, err
	}
	if err := e.store.PersistTrades(ctx, result.Trades); err != nil {
		return nil, err
	}

	e.marketData.Record(req.Symbol, result.Trades)

	orderAcceptedEvent := domain.AuditEvent{
		ID:   domain.NewEventID(),
		Type: "order.accepted",
		Payload: map[string]any{
			"order_id":    order.ID,
			"account_id":  order.AccountID,
			"symbol":      order.Symbol,
			"side":        order.Side,
			"price":       order.Price,
			"quantity":    order.Quantity,
			"remaining":   order.Remaining,
			"status":      order.Status,
			"trades":      len(result.Trades),
			"reserved":    order.ReservedAmount,
			"sequence_no": order.Sequence,
		},
		CreatedAt: time.Now().UTC(),
	}
	if err := e.recordAudit(ctx, orderAcceptedEvent); err != nil {
		return nil, err
	}

	for _, trade := range result.Trades {
		tradeEvent := domain.AuditEvent{
			ID:   domain.NewEventID(),
			Type: "trade.executed",
			Payload: map[string]any{
				"trade_id":          trade.ID,
				"symbol":            trade.Symbol,
				"price":             trade.Price,
				"quantity":          trade.Quantity,
				"maker_order_id":    trade.MakerOrderID,
				"taker_order_id":    trade.TakerOrderID,
				"maker_account_id":  trade.MakerAccountID,
				"taker_account_id":  trade.TakerAccountID,
				"accepted_order_id": order.ID,
			},
			CreatedAt: time.Now().UTC(),
		}
		if err := e.recordAudit(ctx, tradeEvent); err != nil {
			return nil, err
		}
		if err := e.bus.Publish(ctx, TopicTrades, trade.ID, trade); err != nil {
			return nil, err
		}
	}

	bookSnapshot := e.books[req.Symbol].Snapshot()
	if err := e.cache.SetOrderBook(ctx, req.Symbol, bookSnapshot); err != nil {
		return nil, err
	}
	if err := e.cache.SetTrades(ctx, req.Symbol, e.marketData.Trades(req.Symbol)); err != nil {
		return nil, err
	}

	return map[string]any{
		"order":     result.Accepted,
		"trades":    result.Trades,
		"account":   e.wallet.GetAccount(order.AccountID),
		"orderbook": bookSnapshot,
	}, nil
}

func (e *Exchange) GetAccount(ctx context.Context, accountID string) (*domain.Account, error) {
	account, ok, err := e.cache.GetAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if ok {
		return account, nil
	}

	account = e.wallet.GetAccount(accountID)
	if err := e.cache.SetAccount(ctx, account); err != nil {
		return nil, err
	}
	return account, nil
}

func (e *Exchange) OrderBook(ctx context.Context, symbol string) (map[string]any, error) {
	if _, ok := e.symbols[symbol]; !ok {
		return nil, fmt.Errorf("unsupported symbol %s", symbol)
	}

	if snapshot, ok, err := e.cache.GetOrderBook(ctx, symbol); err != nil {
		return nil, err
	} else if ok {
		return snapshot, nil
	}

	snapshot := e.books[symbol].Snapshot()
	if err := e.cache.SetOrderBook(ctx, symbol, snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (e *Exchange) Trades(ctx context.Context, symbol string) ([]domain.Trade, error) {
	if _, ok := e.symbols[symbol]; !ok {
		return nil, fmt.Errorf("unsupported symbol %s", symbol)
	}

	if trades, ok, err := e.cache.GetTrades(ctx, symbol); err != nil {
		return nil, err
	} else if ok {
		return trades, nil
	}

	trades := e.marketData.Trades(symbol)
	if err := e.cache.SetTrades(ctx, symbol, trades); err != nil {
		return nil, err
	}
	return trades, nil
}

func (e *Exchange) Audit() []domain.AuditEvent {
	e.auditMu.RLock()
	defer e.auditMu.RUnlock()
	out := make([]domain.AuditEvent, len(e.audit))
	copy(out, e.audit)
	return out
}

func (e *Exchange) applyTrade(fill engine.Fill, symbol domain.Symbol, accepted *domain.Order) error {
	maker := fill.Maker
	trade := fill.Trade

	if err := e.wallet.Settle(symbol, maker, trade.Price, trade.Quantity); err != nil {
		return err
	}
	if err := e.wallet.Settle(symbol, accepted, trade.Price, trade.Quantity); err != nil {
		return err
	}

	if maker.Side == domain.BuySide {
		e.wallet.AdjustRemainingReservation(maker, maker.Price*trade.Quantity)
	} else {
		e.wallet.AdjustRemainingReservation(maker, trade.Quantity)
	}

	if accepted.Side == domain.BuySide {
		e.wallet.AdjustRemainingReservation(accepted, accepted.Price*trade.Quantity)
	} else {
		e.wallet.AdjustRemainingReservation(accepted, trade.Quantity)
	}

	e.updateOrderStatus(maker)
	e.updateOrderStatus(accepted)

	if maker.Remaining == 0 {
		if err := e.wallet.ReleaseRemaining(symbol, maker); err != nil {
			return err
		}
	}

	return nil
}

func (e *Exchange) updateOrderStatus(order *domain.Order) {
	switch {
	case order.Remaining == 0:
		order.Status = domain.OrderStatusFilled
	case order.Remaining < order.Quantity:
		order.Status = domain.OrderStatusPartiallyFilled
	default:
		order.Status = domain.OrderStatusOpen
	}
	order.UpdatedAt = time.Now().UTC()
}

func (e *Exchange) persistChangedAccounts(ctx context.Context, accounts map[string]*domain.Account) error {
	for _, account := range accounts {
		if err := e.syncAccount(ctx, account); err != nil {
			return err
		}
	}
	return nil
}

func (e *Exchange) persistChangedOrders(ctx context.Context, orders map[string]*domain.Order) error {
	for _, order := range orders {
		if err := e.store.PersistOrder(ctx, order); err != nil {
			return err
		}
		if err := e.bus.Publish(ctx, TopicOrders, order.ID, order); err != nil {
			return err
		}
	}
	return nil
}

func (e *Exchange) syncAccount(ctx context.Context, account *domain.Account) error {
	if err := e.store.PersistAccount(ctx, account); err != nil {
		return err
	}
	e.wallet.ReplaceAccount(account)
	e.syncAccountCache(ctx, account)
	return nil
}

func (e *Exchange) syncAccountCache(ctx context.Context, account *domain.Account) {
	if err := e.cache.SetAccount(ctx, account); err != nil {
		log.Printf("cache set account %s failed: %v", account.ID, err)
	}
}

func (e *Exchange) recordAudit(ctx context.Context, event domain.AuditEvent) error {
	e.auditMu.Lock()
	e.audit = append(e.audit, event)
	e.auditMu.Unlock()

	if err := e.store.PersistAudit(ctx, []domain.AuditEvent{event}); err != nil {
		return err
	}
	return e.bus.Publish(ctx, TopicAudit, event.ID, event)
}

func (e *Exchange) rebuildRecoveredState(ctx context.Context, accounts []*domain.Account, orders []*domain.Order) error {
	reconciledAccounts := e.wallet.RebuildReservations(e.symbols, orders)

	if len(reconciledAccounts) == 0 {
		reconciledAccounts = accounts
	}
	for _, account := range reconciledAccounts {
		if err := e.syncAccount(ctx, account); err != nil {
			return fmt.Errorf("sync recovered account %s: %w", account.ID, err)
		}
	}

	for symbol := range e.symbols {
		if err := e.cache.SetOrderBook(ctx, symbol, e.books[symbol].Snapshot()); err != nil {
			return fmt.Errorf("cache recovered orderbook %s: %w", symbol, err)
		}
		if err := e.cache.SetTrades(ctx, symbol, e.marketData.Trades(symbol)); err != nil {
			return fmt.Errorf("cache recovered trades %s: %w", symbol, err)
		}
	}

	return nil
}
