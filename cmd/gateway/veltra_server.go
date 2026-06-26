package main

// veltra_server.go: endpoints REST de trading da Veltra Exchange.
//
//	POST   /api/orders        - envia ordem (publica order.place em veltra.commands)
//	DELETE /api/orders/{id}   - cancela ordem (publica order.cancel)
//	POST   /api/faucet        - emite saldo virtual (publica faucet.credit)
//	GET    /api/veltra/state  - snapshot das projecoes (book, trades, ordens, saldos)
//
// Precos e quantidades entram como STRING decimal ("1.5") e sao convertidos
// para inteiro escalado com money.Parse — o cliente nunca faz aritmetica de
// dinheiro em float. A validacao de saldo/risco pre-trade (OMS, Fase 3) ainda
// nao existe: por ora as ordens vao direto ao motor.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/ledger"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// marketRoutes registra os endpoints de market data.
func (s *Server) marketRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/market", s.getMarket)
	mux.HandleFunc("/api/market/", s.getMarketCandles) // /api/market/{symbol}/candles
}

func (s *Server) getMarket(w http.ResponseWriter, r *http.Request) {
	body, err := s.veltra.MarketSnapshot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func (s *Server) getMarketCandles(w http.ResponseWriter, r *http.Request) {
	// /api/market/{symbol}/candles
	rest := strings.TrimPrefix(r.URL.Path, "/api/market/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[1] != "candles" || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	symbol := strings.ToUpper(parts[0])
	body, err := s.veltra.CandlesForSymbol(symbol)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// veltraRoutes registra os endpoints da exchange no mux.
// Quando auth está habilitada, orders e faucet exigem JWT válido.
func (s *Server) veltraRoutes(mux *http.ServeMux) {
	handleOrders := http.HandlerFunc(s.handleOrders)
	handleCancel := http.HandlerFunc(s.handleOrderCancel)
	handleFaucet := http.HandlerFunc(s.postFaucet)

	if s.auth != nil {
		handleOrders = s.auth.RequireAuth(s.handleOrders)
		handleCancel = s.auth.RequireAuth(s.handleOrderCancel)
		handleFaucet = s.auth.RequireAuth(s.postFaucet)
	}

	mux.Handle("/api/orders", handleOrders)
	mux.Handle("/api/orders/", handleCancel)
	mux.Handle("/api/faucet", handleFaucet)
	mux.HandleFunc("/api/veltra/state", s.getVeltraState)
	mux.HandleFunc("/api/veltra/merkle", s.getMerkleRoots)
}

// getMerkleRoots expõe a trilha de auditoria: as Merkle roots por período
// computadas pelo ledger (plano §4.3 — provas de auditoria sem expor o razão).
func (s *Server) getMerkleRoots(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeError(w, http.StatusServiceUnavailable, "auditoria indisponivel (sem Postgres)")
		return
	}
	rows, err := s.auth.db.QueryContext(r.Context(),
		`SELECT period_start, period_end, root_hash, posting_count
		 FROM ledger.merkle_roots ORDER BY period_end DESC LIMIT 50`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type merkleRoot struct {
		PeriodStart  string `json:"period_start"`
		PeriodEnd    string `json:"period_end"`
		RootHash     string `json:"root_hash"`
		PostingCount int64  `json:"posting_count"`
	}
	out := []merkleRoot{}
	for rows.Next() {
		var m merkleRoot
		if err := rows.Scan(&m.PeriodStart, &m.PeriodEnd, &m.RootHash, &m.PostingCount); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out = append(out, m)
	}
	writeJSON(w, http.StatusOK, map[string]any{"merkle_roots": out})
}

func (s *Server) getVeltraState(w http.ResponseWriter, r *http.Request) {
	body, err := s.veltra.SnapshotJSON()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// releaseHoldOnTerminal libera a reserva (hold) do OMS quando uma ordem atinge
// estado terminal. Chamado pelo consumer de eventos da Veltra no gateway.
//   - order.filled com remaining=0 → ordem totalmente executada
//   - order.rejected               → recusada no motor
//   - order.canceled               → cancelada (client_order_id resolvido pela projeção)
func releaseHoldOnTerminal(ctx context.Context, l *ledger.Ledger, veltra *VeltraState, rk string, env messaging.Envelope) {
	if l == nil {
		return
	}
	switch rk {
	case messaging.RKOrderFilled:
		var p messaging.OrderFilledPayload
		if env.Unmarshal(&p) == nil && p.RemainingQty == 0 && p.ClientOrderID != "" {
			_ = l.ReleaseReserveByOrder(ctx, p.ClientOrderID)
		}
	case messaging.RKOrderRejected:
		var p messaging.OrderRejectedPayload
		if env.Unmarshal(&p) == nil && p.ClientOrderID != "" {
			_ = l.ReleaseReserveByOrder(ctx, p.ClientOrderID)
		}
	case messaging.RKOrderCanceled:
		var p messaging.OrderCanceledPayload
		if env.Unmarshal(&p) == nil {
			if coid := veltra.ClientOrderIDFor(p.OrderID); coid != "" {
				_ = l.ReleaseReserveByOrder(ctx, coid)
			}
		}
	}
}

type orderReq struct {
	Account       string `json:"account"`
	Pair          string `json:"pair"`
	Side          string `json:"side"`            // buy | sell
	Type          string `json:"type"`            // limit | market
	TimeInForce   string `json:"time_in_force"`   // gtc | ioc | fok (default gtc; market -> ioc)
	Price         string `json:"price"`           // decimal string; vazio para market
	Quantity      string `json:"quantity"`        // decimal string
	ClientOrderID string `json:"client_order_id"` // opcional; default = tx_id
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req orderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}

	// Quando auth está ativa, a conta vem do token JWT — ignora payload.
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		req.Account = claims.Username
	}

	if !accountRegex.MatchString(req.Account) {
		writeError(w, http.StatusBadRequest, "conta invalida")
		return
	}
	if _, err := exchange.ParsePair(req.Pair); err != nil {
		writeError(w, http.StatusBadRequest, "par invalido")
		return
	}
	if req.Side != "buy" && req.Side != "sell" {
		writeError(w, http.StatusBadRequest, "side deve ser buy ou sell")
		return
	}
	if req.Type != "limit" && req.Type != "market" {
		writeError(w, http.StatusBadRequest, "type deve ser limit ou market")
		return
	}

	qty, err := money.Parse(req.Quantity)
	if err != nil || !qty.IsPositive() {
		writeError(w, http.StatusBadRequest, "quantity invalida")
		return
	}

	var price money.Amount
	if req.Type == "limit" {
		price, err = money.Parse(req.Price)
		if err != nil || !price.IsPositive() {
			writeError(w, http.StatusBadRequest, "price invalido para ordem limit")
			return
		}
	}

	tif := strings.ToLower(req.TimeInForce)
	switch tif {
	case "gtc", "ioc", "fok":
	case "":
		if req.Type == "market" {
			tif = "ioc"
		} else {
			tif = "gtc"
		}
	default:
		writeError(w, http.StatusBadRequest, "time_in_force deve ser gtc, ioc ou fok")
		return
	}

	// Apenas pares com quote USDT-sim são suportados (VLT/USDT-sim + catálogo).
	parsed, _ := exchange.ParsePair(req.Pair) // já validado acima
	if string(parsed.Quote) != "USDT-sim" {
		writeError(w, http.StatusBadRequest, "apenas pares com quote USDT-sim sao suportados")
		return
	}
	baseAsset := string(parsed.Base)
	quoteAsset := string(parsed.Quote)

	// Monta o comando order.place e fixa o client_order_id (idempotência) ANTES
	// da reserva — a reserva e o release do hold são chaveados por ele.
	payload := messaging.OrderPlacePayload{
		ClientOrderID: req.ClientOrderID,
		Account:       req.Account,
		Pair:          req.Pair,
		Side:          req.Side,
		Type:          req.Type,
		TimeInForce:   tif,
		Price:         int64(price),
		Quantity:      int64(qty),
	}
	env, err := messaging.NewEnvelope(messaging.SchemaOrderPlace, "", payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if payload.ClientOrderID == "" {
		payload.ClientOrderID = env.TxID
		env, _ = messaging.NewEnvelope(messaging.SchemaOrderPlace, env.TxID, payload)
	}

	// ── OMS pré-trade: RESERVA atômica do saldo antes de enfileirar a ordem ──
	// A reserva condicional (UPDATE ... WHERE balance-reserved >= amount) decide
	// e aplica numa única instrução, sem janela de corrida. Liberada no evento
	// terminal da ordem (filled/canceled/rejected) pelo consumer de eventos.
	if claims, ok := ClaimsFromContext(r.Context()); ok && s.ledger != nil {
		var reserveAsset string
		var reserveAmt int64
		var reason string
		if req.Side == "sell" {
			// Venda: reserva a quantidade do ativo base.
			reserveAsset, reserveAmt, reason = baseAsset, int64(qty), "sell_quantity"
		} else {
			// Compra: reserva o notional em quote (preço × qtd, sem truncar).
			var refPrice money.Amount
			if req.Type == "limit" && price.IsPositive() {
				refPrice = price
			} else {
				refPrice = money.Amount(s.veltra.MarketPriceFor(baseAsset))
			}
			if refPrice.IsPositive() {
				notional, nerr := money.Notional(refPrice, qty)
				if nerr != nil {
					writeError(w, http.StatusBadRequest, "notional invalido: "+nerr.Error())
					return
				}
				reserveAsset, reserveAmt, reason = quoteAsset, int64(notional), "buy_notional"
			}
		}
		if reserveAmt > 0 {
			ok, rerr := s.ledger.ReserveIfAvailable(r.Context(), claims.AccountID, reserveAsset, payload.ClientOrderID, reserveAmt, reason)
			if rerr != nil {
				log.Printf("[Gateway] OMS reserve(%s): %v", payload.ClientOrderID, rerr)
				writeError(w, http.StatusServiceUnavailable, "erro ao reservar saldo")
				return
			}
			if !ok {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("saldo insuficiente de %s para a ordem", reserveAsset))
				return
			}
		}
	}

	// Publica o comando: TODOS os pares passam pelo matching engine real
	// (CLOB determinístico + WAL), que é a única fonte da verdade (plano §4.3/4.4).
	if err := s.publisher.Publish(r.Context(), messaging.ExchangeVeltraCommands, messaging.RKOrderPlace, env, nil); err != nil {
		log.Printf("[Gateway] Falha ao publicar order.place: %v", err)
		// Desfaz a reserva: a ordem não entrou.
		if s.ledger != nil {
			_ = s.ledger.ReleaseReserveByOrder(r.Context(), payload.ClientOrderID)
		}
		writeError(w, http.StatusServiceUnavailable, "broker indisponivel")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":          "queued",
		"tx_id":           env.TxID,
		"client_order_id": payload.ClientOrderID,
	})
}

type cancelReq struct {
	Account string `json:"account"`
	Pair    string `json:"pair"`
}

func (s *Server) handleOrderCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "use DELETE", http.StatusMethodNotAllowed)
		return
	}
	orderID := strings.TrimPrefix(r.URL.Path, "/api/orders/")
	if orderID == "" || strings.Contains(orderID, "/") {
		writeError(w, http.StatusBadRequest, "order id invalido")
		return
	}
	var req cancelReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		req.Account = claims.Username
	}
	if _, err := exchange.ParsePair(req.Pair); err != nil {
		writeError(w, http.StatusBadRequest, "par invalido")
		return
	}

	payload := messaging.OrderCancelPayload{
		OrderID: orderID,
		Account: req.Account,
		Pair:    req.Pair,
	}
	env, err := messaging.NewEnvelope(messaging.SchemaOrderCancel, "", payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.publisher.Publish(r.Context(), messaging.ExchangeVeltraCommands, messaging.RKOrderCancel, env, nil); err != nil {
		log.Printf("[Gateway] Falha ao publicar order.cancel: %v", err)
		writeError(w, http.StatusServiceUnavailable, "broker indisponivel")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "tx_id": env.TxID})
}

type faucetReq struct {
	Account string `json:"account"`
	Asset   string `json:"asset"`
	Amount  string `json:"amount"` // decimal string
}

// postFaucet emite credito virtual (modelo "adicionar credito" do projeto base,
// plano secao 4.3.1). Publicado como EVENTO: a emissao do admin e soberana e
// nao requer processamento previo. Quando o Ledger (Fase 2) existir, ele
// consumira este mesmo evento para o lancamento de dupla entrada.
func (s *Server) postFaucet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return
	}
	var req faucetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido: "+err.Error())
		return
	}
	if claims, ok := ClaimsFromContext(r.Context()); ok {
		req.Account = claims.Username
	}
	if !accountRegex.MatchString(req.Account) {
		writeError(w, http.StatusBadRequest, "conta invalida")
		return
	}
	if req.Asset == "" {
		writeError(w, http.StatusBadRequest, "asset obrigatorio")
		return
	}
	amount, err := money.Parse(req.Amount)
	if err != nil || !amount.IsPositive() {
		writeError(w, http.StatusBadRequest, "amount invalido")
		return
	}

	payload := messaging.FaucetCreditPayload{
		Account: req.Account,
		Asset:   req.Asset,
		Amount:  int64(amount),
	}
	env, err := messaging.NewEnvelope(messaging.SchemaFaucetCredit, "", payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.publisher.Publish(r.Context(), messaging.ExchangeVeltraEvents, messaging.RKFaucetCredit, env, nil); err != nil {
		log.Printf("[Gateway] Falha ao publicar faucet.credit: %v", err)
		writeError(w, http.StatusServiceUnavailable, "broker indisponivel")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "tx_id": env.TxID})
}
