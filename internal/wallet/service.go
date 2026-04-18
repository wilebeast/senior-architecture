package wallet

import (
	"fmt"
	"sync"

	"senior-architecture/internal/domain"
)

type Service struct {
	mu       sync.RWMutex
	accounts map[string]*domain.Account
}

func NewService(accounts ...*domain.Account) *Service {
	service := &Service{
		accounts: make(map[string]*domain.Account),
	}
	for _, account := range accounts {
		service.accounts[account.ID] = cloneAccount(account)
	}
	return service
}

func (s *Service) Deposit(accountID, asset string, amount float64) (*domain.Account, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	account := s.ensureAccount(accountID)
	balance := account.Balances[asset]
	balance.Available += amount
	account.Balances[asset] = balance

	return cloneAccount(account), nil
}

func (s *Service) Reserve(symbol domain.Symbol, order *domain.Order) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	account := s.ensureAccount(order.AccountID)

	switch order.Side {
	case domain.BuySide:
		required := order.Price * order.Quantity
		balance := account.Balances[symbol.QuoteAsset]
		if balance.Available < required {
			return fmt.Errorf("insufficient %s balance", symbol.QuoteAsset)
		}
		balance.Available -= required
		balance.Locked += required
		account.Balances[symbol.QuoteAsset] = balance
		order.ReservedAmount = required
	case domain.SellSide:
		required := order.Quantity
		balance := account.Balances[symbol.BaseAsset]
		if balance.Available < required {
			return fmt.Errorf("insufficient %s balance", symbol.BaseAsset)
		}
		balance.Available -= required
		balance.Locked += required
		account.Balances[symbol.BaseAsset] = balance
		order.ReservedAmount = required
	default:
		return fmt.Errorf("unsupported side")
	}

	return nil
}

func (s *Service) Settle(symbol domain.Symbol, order *domain.Order, tradePrice, tradeQty float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	account := s.ensureAccount(order.AccountID)

	switch order.Side {
	case domain.BuySide:
		quoteLocked := account.Balances[symbol.QuoteAsset]
		reservedCost := order.Price * tradeQty
		actualCost := tradePrice * tradeQty
		refund := reservedCost - actualCost
		if quoteLocked.Locked < reservedCost {
			return fmt.Errorf("locked %s underflow", symbol.QuoteAsset)
		}
		quoteLocked.Locked -= reservedCost
		quoteLocked.Available += refund
		account.Balances[symbol.QuoteAsset] = quoteLocked

		baseBalance := account.Balances[symbol.BaseAsset]
		baseBalance.Available += tradeQty
		account.Balances[symbol.BaseAsset] = baseBalance
	case domain.SellSide:
		baseLocked := account.Balances[symbol.BaseAsset]
		if baseLocked.Locked < tradeQty {
			return fmt.Errorf("locked %s underflow", symbol.BaseAsset)
		}
		baseLocked.Locked -= tradeQty
		account.Balances[symbol.BaseAsset] = baseLocked

		quoteBalance := account.Balances[symbol.QuoteAsset]
		quoteBalance.Available += tradePrice * tradeQty
		account.Balances[symbol.QuoteAsset] = quoteBalance
	default:
		return fmt.Errorf("unsupported side")
	}

	return nil
}

func (s *Service) ReleaseRemaining(symbol domain.Symbol, order *domain.Order) error {
	if order.Remaining > 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	account := s.ensureAccount(order.AccountID)

	switch order.Side {
	case domain.BuySide:
		balance := account.Balances[symbol.QuoteAsset]
		if balance.Locked < order.ReservedAmount {
			order.ReservedAmount = balance.Locked
		}
		balance.Locked -= order.ReservedAmount
		balance.Available += order.ReservedAmount
		account.Balances[symbol.QuoteAsset] = balance
		order.ReservedAmount = 0
	case domain.SellSide:
		balance := account.Balances[symbol.BaseAsset]
		if balance.Locked < order.ReservedAmount {
			order.ReservedAmount = balance.Locked
		}
		balance.Locked -= order.ReservedAmount
		balance.Available += order.ReservedAmount
		account.Balances[symbol.BaseAsset] = balance
		order.ReservedAmount = 0
	}

	return nil
}

func (s *Service) AdjustRemainingReservation(order *domain.Order, consumed float64) {
	order.ReservedAmount -= consumed
	if order.ReservedAmount < 0 {
		order.ReservedAmount = 0
	}
}

func (s *Service) GetAccount(accountID string) *domain.Account {
	s.mu.RLock()
	defer s.mu.RUnlock()

	account, ok := s.accounts[accountID]
	if !ok {
		return &domain.Account{ID: accountID, Balances: map[string]domain.Balance{}}
	}
	return cloneAccount(account)
}

func (s *Service) RebuildReservations(symbols map[string]domain.Symbol, orders []*domain.Order) []*domain.Account {
	s.mu.Lock()
	defer s.mu.Unlock()

	type assetKey struct {
		accountID string
		asset     string
	}

	affected := make(map[assetKey]struct{})
	for _, order := range orders {
		symbol, ok := symbols[order.Symbol]
		if !ok {
			continue
		}
		account := s.ensureAccount(order.AccountID)
		switch order.Side {
		case domain.BuySide:
			key := assetKey{accountID: order.AccountID, asset: symbol.QuoteAsset}
			balance := account.Balances[symbol.QuoteAsset]
			balance.Available += balance.Locked
			balance.Locked = 0
			account.Balances[symbol.QuoteAsset] = balance
			affected[key] = struct{}{}
		case domain.SellSide:
			key := assetKey{accountID: order.AccountID, asset: symbol.BaseAsset}
			balance := account.Balances[symbol.BaseAsset]
			balance.Available += balance.Locked
			balance.Locked = 0
			account.Balances[symbol.BaseAsset] = balance
			affected[key] = struct{}{}
		}
	}

	for _, order := range orders {
		symbol, ok := symbols[order.Symbol]
		if !ok {
			continue
		}
		account := s.ensureAccount(order.AccountID)
		switch order.Side {
		case domain.BuySide:
			balance := account.Balances[symbol.QuoteAsset]
			lockAmount := order.ReservedAmount
			if lockAmount <= 0 {
				lockAmount = order.Price * order.Remaining
			}
			if lockAmount > balance.Available {
				lockAmount = balance.Available
			}
			balance.Available -= lockAmount
			balance.Locked += lockAmount
			account.Balances[symbol.QuoteAsset] = balance
			order.ReservedAmount = lockAmount
		case domain.SellSide:
			balance := account.Balances[symbol.BaseAsset]
			lockAmount := order.ReservedAmount
			if lockAmount <= 0 {
				lockAmount = order.Remaining
			}
			if lockAmount > balance.Available {
				lockAmount = balance.Available
			}
			balance.Available -= lockAmount
			balance.Locked += lockAmount
			account.Balances[symbol.BaseAsset] = balance
			order.ReservedAmount = lockAmount
		}
	}

	accounts := make([]*domain.Account, 0, len(affected))
	seen := make(map[string]struct{})
	for key := range affected {
		if _, ok := seen[key.accountID]; ok {
			continue
		}
		seen[key.accountID] = struct{}{}
		accounts = append(accounts, cloneAccount(s.accounts[key.accountID]))
	}
	return accounts
}

func (s *Service) ensureAccount(accountID string) *domain.Account {
	account, ok := s.accounts[accountID]
	if !ok {
		account = &domain.Account{
			ID:       accountID,
			Balances: make(map[string]domain.Balance),
		}
		s.accounts[accountID] = account
	}
	return account
}

func cloneAccount(account *domain.Account) *domain.Account {
	cloned := &domain.Account{
		ID:       account.ID,
		Balances: make(map[string]domain.Balance, len(account.Balances)),
	}
	for asset, balance := range account.Balances {
		cloned.Balances[asset] = balance
	}
	return cloned
}
