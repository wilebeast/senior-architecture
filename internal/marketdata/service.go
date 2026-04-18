package marketdata

import (
	"sync"

	"senior-architecture/internal/domain"
)

type Service struct {
	mu     sync.RWMutex
	trades map[string][]domain.Trade
}

func NewService() *Service {
	return &Service{
		trades: make(map[string][]domain.Trade),
	}
}

func (s *Service) Load(trades []domain.Trade) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, trade := range trades {
		s.trades[trade.Symbol] = append(s.trades[trade.Symbol], trade)
	}
}

func (s *Service) Record(symbol string, trades []domain.Trade) {
	if len(trades) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.trades[symbol] = append(s.trades[symbol], trades...)
}

func (s *Service) Trades(symbol string) []domain.Trade {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trades := s.trades[symbol]
	out := make([]domain.Trade, len(trades))
	copy(out, trades)
	return out
}
