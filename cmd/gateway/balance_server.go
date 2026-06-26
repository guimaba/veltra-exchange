package main

// balance_server.go: endpoints de saldo e admin da Veltra Exchange.
//
//	GET  /api/balance           — saldo do usuário autenticado (fonte: Postgres ledger)
//	POST /api/deposit           — simula depósito (credita a fiat USD/BRL/EUR via faucet)
//	GET  /api/admin/users       — lista todos os usuários com saldos (admin only)
//	GET  /api/admin/stats       — estatísticas do sistema (admin only)

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

func (s *Server) balanceRoutes(mux *http.ServeMux) {
	if s.auth == nil {
		return
	}
	mux.Handle("/api/balance", s.auth.RequireAuth(s.handleBalance))
	mux.Handle("/api/deposit", s.auth.RequireAuth(s.handleDeposit))
	mux.Handle("/api/withdraw", s.auth.RequireAuth(s.handleWithdraw))
	mux.Handle("/api/admin/users", s.auth.RequireAuth(s.handleAdminUsers))
	mux.Handle("/api/admin/stats", s.auth.RequireAuth(s.handleAdminStats))
	mux.Handle("/api/admin/credit", s.auth.RequireAuth(s.handleAdminCredit))
	mux.Handle("/api/admin/promote", s.auth.RequireAuth(s.handleAdminPromote))
}

// fiatRates: unidades da moeda por 1 USD. Taxas SIMULADAS e fixas — nenhum valor
// é real. São as moedas de cotação (USD/BRL/EUR): cada uma é um ativo próprio,
// usado tanto para depósito/saque quanto como lado QUOTE dos pares de trading.
var fiatRates = map[string]float64{
	"USD": 1.00,
	"BRL": 5.20,
	"EUR": 0.92,
}

func fiatRate(code string) (float64, bool) {
	r, ok := fiatRates[strings.ToUpper(code)]
	return r, ok
}

// handleBalance retorna os saldos do usuário autenticado do ledger Postgres.
func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	rows, err := s.auth.db.QueryContext(r.Context(), `
		SELECT la.asset, la.balance, la.reserved, la.available
		FROM ledger.accounts la
		WHERE la.trading_account_id = $1 AND la.balance > 0
		ORDER BY la.asset
	`, claims.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao consultar saldo")
		return
	}
	defer rows.Close()

	type BalEntry struct {
		Asset     string `json:"asset"`
		Balance   int64  `json:"balance"`
		Reserved  int64  `json:"reserved"`
		Available int64  `json:"available"`
	}
	var entries []BalEntry
	for rows.Next() {
		var e BalEntry
		if err := rows.Scan(&e.Asset, &e.Balance, &e.Reserved, &e.Available); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []BalEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id": claims.AccountID,
		"username":   claims.Username,
		"balances":   entries,
	})
}

// handleDeposit simula um depósito: credita a moeda fiat (USD/BRL/EUR) 1:1 para
// o usuário autenticado.
func (s *Server) handleDeposit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	var req struct {
		Amount   string `json:"amount"`
		Method   string `json:"method"`
		Currency string `json:"currency"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if req.Amount == "" {
		writeError(w, http.StatusBadRequest, "amount obrigatório")
		return
	}

	currency := strings.ToUpper(req.Currency)
	if currency == "" {
		currency = "BRL"
	}
	if _, ok := fiatRate(currency); !ok {
		writeError(w, http.StatusBadRequest, "moeda não suportada: "+currency)
		return
	}

	amount, err := money.Parse(req.Amount)
	if err != nil || !amount.IsPositive() {
		writeError(w, http.StatusBadRequest, "valor inválido")
		return
	}

	// A moeda fiat é um ativo próprio: deposita R$/US$/€ → credita 1:1 esse
	// ativo. As criptos só se obtêm negociando contra essa fiat.
	txID, err := s.publishFaucet(r.Context(), claims.Username, currency, int64(amount))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "erro ao processar depósito")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"tx_id":    txID,
		"amount":   req.Amount,
		"currency": currency,
		"asset":    currency,
		"method":   req.Method,
	})
}

// handleWithdraw simula um saque: debita uma moeda fiat (USD/BRL/EUR) 1:1 e
// devolve o valor como cash-out. Nenhum valor real existe — é só a simulação.
func (s *Server) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	claims, _ := ClaimsFromContext(r.Context())

	var req struct {
		Asset    string `json:"asset"`
		Amount   string `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	asset := strings.ToUpper(req.Asset)
	// Só moedas fiat podem ser sacadas (cripto só se mantém ou negocia).
	if _, ok := fiatRate(asset); !ok {
		writeError(w, http.StatusBadRequest, "só é possível sacar moedas fiat (USD/BRL/EUR). Converta cripto na aba de trading.")
		return
	}
	amount, err := money.Parse(req.Amount)
	if err != nil || !amount.IsPositive() {
		writeError(w, http.StatusBadRequest, "valor inválido")
		return
	}

	// Valida que o usuário TEM essa moeda (não dá pra sacar real só tendo dólar).
	var available int64
	err = s.auth.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(available, 0) FROM ledger.accounts WHERE trading_account_id = $1 AND asset = $2`,
		claims.AccountID, asset).Scan(&available)
	if err != nil || available < int64(amount) {
		writeError(w, http.StatusBadRequest, "saldo insuficiente de "+asset)
		return
	}

	// Saque 1:1 na própria moeda (debita o ativo fiat — sai do sistema).
	txID, err := s.publishFaucet(r.Context(), claims.Username, asset, -int64(amount))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "erro ao processar saque")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "success",
		"tx_id":       txID,
		"asset":       asset,
		"amount":      req.Amount,
		"currency":    asset,
		"payout_fiat": float64(amount) / 1e8,
	})
}

// publishFaucet publica faucet.credit no RabbitMQ e retorna o txID.
func (s *Server) publishFaucet(ctx context.Context, account, asset string, amount int64) (string, error) {
	payload := messaging.FaucetCreditPayload{Account: account, Asset: asset, Amount: amount}
	env, err := messaging.NewEnvelope(messaging.SchemaFaucetCredit, "", payload)
	if err != nil {
		return "", fmt.Errorf("NewEnvelope: %w", err)
	}
	if err := s.publisher.Publish(ctx, messaging.ExchangeVeltraEvents, messaging.RKFaucetCredit, env, nil); err != nil {
		log.Printf("[Gateway] publishFaucet: %v", err)
		return "", err
	}
	return env.TxID, nil
}

// handleAdminUsers lista todos os usuários com seus saldos (admin only).
func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	claims, _ := ClaimsFromContext(r.Context())
	if !claims.IsAdmin {
		writeError(w, http.StatusForbidden, "acesso restrito a administradores")
		return
	}

	rows, err := s.auth.db.QueryContext(r.Context(), `
		SELECT
			u.id, u.username, COALESCE(u.email,'') AS email,
			u.is_admin, u.created_at,
			COALESCE(
				json_agg(
					json_build_object('asset', la.asset, 'balance', la.balance)
					ORDER BY la.asset
				) FILTER (WHERE la.asset IS NOT NULL AND la.balance > 0),
				'[]'::json
			) AS balances
		FROM auth.users u
		LEFT JOIN auth.accounts aa ON aa.user_id = u.id
		LEFT JOIN ledger.accounts la ON la.trading_account_id = aa.id AND la.balance > 0
		WHERE u.id > 0
		  AND u.username NOT IN ('system', '_market')
		GROUP BY u.id, u.username, u.email, u.is_admin, u.created_at
		ORDER BY u.created_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao consultar usuários")
		return
	}
	defer rows.Close()

	type UserRow struct {
		ID        int64           `json:"id"`
		Username  string          `json:"username"`
		Email     string          `json:"email"`
		IsAdmin   bool            `json:"is_admin"`
		CreatedAt string          `json:"created_at"`
		Balances  json.RawMessage `json:"balances"`
	}
	var users []UserRow
	for rows.Next() {
		var u UserRow
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.IsAdmin, &createdAt, &u.Balances); err != nil {
			continue
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		users = append(users, u)
	}
	if users == nil {
		users = []UserRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users, "total": len(users)})
}

// handleAdminStats retorna estatísticas do sistema.
func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "use GET", http.StatusMethodNotAllowed)
		return
	}
	claims, _ := ClaimsFromContext(r.Context())
	if !claims.IsAdmin {
		writeError(w, http.StatusForbidden, "acesso restrito a administradores")
		return
	}
	ctx := r.Context()

	var totalUsers, totalTrades, totalFaucets int64
	var totalVolume float64

	s.auth.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM auth.users WHERE id > 0 AND username NOT IN ('system','_market')`).Scan(&totalUsers)
	s.auth.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ledger.postings WHERE operation_type = 'trade' AND amount > 0`).Scan(&totalTrades)
	s.auth.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM ledger.postings WHERE operation_type = 'faucet' AND amount > 0`).Scan(&totalFaucets)
	// Volume NEGOCIADO (só trades) — não inflar com emissão de faucet.
	s.auth.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount::numeric / 100000000.0), 0) FROM ledger.postings
		 WHERE operation_type = 'trade' AND amount > 0`).Scan(&totalVolume)

	writeJSON(w, http.StatusOK, map[string]any{
		"total_users":   totalUsers,
		"total_trades":  totalTrades / 2,
		"total_faucets": totalFaucets,
		"total_volume":  totalVolume,
		"ws_clients":    s.hub.ClientCount(),
		"leader":        s.state.Leader(),
	})
}

// handleAdminCredit ajusta o saldo de um usuário (credita ou debita qualquer
// ativo). amount pode ser negativo (debita). Admin only.
func (s *Server) handleAdminCredit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	claims, _ := ClaimsFromContext(r.Context())
	if !claims.IsAdmin {
		writeError(w, http.StatusForbidden, "acesso restrito a administradores")
		return
	}
	var req struct {
		Username string `json:"username"`
		Asset    string `json:"asset"`
		Amount   string `json:"amount"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if req.Username == "" || req.Asset == "" {
		writeError(w, http.StatusBadRequest, "username e asset obrigatórios")
		return
	}

	// amount com sinal: "-100" debita.
	negative := strings.HasPrefix(strings.TrimSpace(req.Amount), "-")
	abs := strings.TrimPrefix(strings.TrimSpace(req.Amount), "-")
	amount, err := money.Parse(abs)
	if err != nil || !amount.IsPositive() {
		writeError(w, http.StatusBadRequest, "valor inválido")
		return
	}
	delta := int64(amount)
	if negative {
		delta = -delta
	}

	txID, err := s.publishFaucet(r.Context(), req.Username, req.Asset, delta)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "erro ao ajustar saldo")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success", "tx_id": txID})
}

// handleAdminPromote promove/rebaixa um usuário a admin. Admin only.
func (s *Server) handleAdminPromote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	claims, _ := ClaimsFromContext(r.Context())
	if !claims.IsAdmin {
		writeError(w, http.StatusForbidden, "acesso restrito a administradores")
		return
	}
	var req struct {
		Username string `json:"username"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "username obrigatório")
		return
	}
	_, err := s.auth.db.ExecContext(r.Context(),
		`UPDATE auth.users SET is_admin = $1, updated_at = CURRENT_TIMESTAMP WHERE username = $2`,
		req.IsAdmin, req.Username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao atualizar usuário")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "success"})
}

// CheckSufficientBalance verifica se a conta tem saldo disponível suficiente
// para a operação. Retorna erro descritivo se insuficiente.
//
//	side="buy"  → precisa de 'required' de quoteAsset (USD/BRL/EUR)
//	side="sell" → precisa de 'required' de baseAsset (o token)
func (s *Server) CheckSufficientBalance(ctx context.Context, accountID int64, asset string, required int64) error {
	if s.auth == nil {
		return nil // sem Postgres configurado, permite (modo demo)
	}
	var available int64
	err := s.auth.db.QueryRowContext(ctx,
		`SELECT COALESCE(la.available, 0)
		 FROM ledger.accounts la
		 WHERE la.trading_account_id = $1 AND la.asset = $2`,
		accountID, asset,
	).Scan(&available)
	if err != nil {
		// Conta pode não existir ainda (primeiro trade) — permite mas não bloqueia
		return nil
	}
	if available < required {
		avlDec := float64(available) / 1e8
		reqDec := float64(required) / 1e8
		return fmt.Errorf("saldo insuficiente: disponível %.6f %s, necessário %.6f %s",
			avlDec, asset, reqDec, asset)
	}
	return nil
}

// ===== helpers =====

// formatTime converte interface{} (time.Time do postgres) para string ISO.
func formatTime(v interface{}) string {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	return fmt.Sprint(v)
}

// ── usado por veltra_server.go para o depósito ──
var _ = uuid.New
var _ = strconv.Itoa
var _ = strings.TrimSpace
