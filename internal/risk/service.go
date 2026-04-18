package risk

import (
	"fmt"

	"senior-architecture/internal/domain"
	"senior-architecture/internal/wallet"
)

type Service struct {
	wallet *wallet.Service
}

func NewService(walletService *wallet.Service) *Service {
	return &Service{wallet: walletService}
}

func (s *Service) ValidateOrder(symbol domain.Symbol, order *domain.Order) error {
	if order.Price <= 0 {
		return fmt.Errorf("price must be positive")
	}
	if order.Quantity <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	if order.Symbol != symbol.Name {
		return fmt.Errorf("symbol mismatch")
	}

	account := s.wallet.GetAccount(order.AccountID)
	switch order.Side {
	case domain.BuySide:
		required := order.Price * order.Quantity
		if account.Balances[symbol.QuoteAsset].Available < required {
			return fmt.Errorf("insufficient available %s", symbol.QuoteAsset)
		}
	case domain.SellSide:
		if account.Balances[symbol.BaseAsset].Available < order.Quantity {
			return fmt.Errorf("insufficient available %s", symbol.BaseAsset)
		}
	default:
		return fmt.Errorf("unsupported side")
	}

	return nil
}
