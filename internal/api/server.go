package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"senior-architecture/internal/domain"
	"senior-architecture/internal/platform"
)

type Server struct {
	exchange *platform.Exchange
}

func NewServer(exchange *platform.Exchange) *Server {
	return &Server{exchange: exchange}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/accounts/deposit", s.handleDeposit)
	mux.HandleFunc("/v1/accounts/", s.handleAccount)
	mux.HandleFunc("/v1/orders", s.handleOrder)
	mux.HandleFunc("/v1/orderbook/", s.handleOrderBook)
	mux.HandleFunc("/v1/trades/", s.handleTrades)
	mux.HandleFunc("/v1/audit", s.handleAudit)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	var req domain.DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	account, err := s.exchange.Deposit(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, account)
}

func (s *Server) handleAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	accountID := strings.TrimPrefix(r.URL.Path, "/v1/accounts/")
	account, err := s.exchange.GetAccount(r.Context(), accountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) handleOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}

	var req domain.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	result, err := s.exchange.PlaceOrder(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOrderBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/v1/orderbook/")
	book, err := s.exchange.OrderBook(r.Context(), symbol)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (s *Server) handleTrades(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	symbol := strings.TrimPrefix(r.URL.Path, "/v1/trades/")
	trades, err := s.exchange.Trades(r.Context(), symbol)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, trades)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	writeJSON(w, http.StatusOK, s.exchange.Audit())
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
