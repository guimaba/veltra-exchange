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
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// simTradeSeq e o contador atomico global de sequencia para trades simulados.
var simTradeSeq uint64

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

// simulateTrade executa um fill imediato simulado para pares nao-VLT.
// Publica order.accepted, order.filled e trade.executed no exchange de eventos.
// Retorna o orderID gerado e qualquer erro de publicacao.
func (s *Server) simulateTrade(
	ctx context.Context,
	account, pair, side, orderType string,
	quantity int64,
	limitPrice int64,
	clientOrderID string,
) (orderID string, fillPrice int64, err error) {
	symbol, _ := func() (string, string) {
		if idx := strings.Index(pair, "/"); idx >= 0 {
			return pair[:idx], pair[idx+1:]
		}
		return pair, ""
	}()

	// Determina preco de fill: usa o preco corrente de mercado sempre.
	fillPrice = s.veltra.MarketPriceFor(symbol)
	if fillPrice == 0 {
		// Simbolo desconhecido mas aceito; usa limitPrice ou rejeita
		if limitPrice > 0 {
			fillPrice = limitPrice
		} else {
			return "", 0, fmt.Errorf("preco de mercado indisponivel para %s", symbol)
		}
	}

	orderID = uuid.NewString()
	tradeID := uuid.NewString()
	seq := atomic.AddUint64(&simTradeSeq, 1)
	tradeIDStr := fmt.Sprintf("%s-sim-%s", symbol, tradeID[:8])
	nowMs := time.Now().UnixMilli()

	// 1. order.accepted
	acceptedPayload := messaging.OrderAcceptedPayload{
		OrderID:       orderID,
		ClientOrderID: clientOrderID,
		Account:       account,
		Pair:          pair,
		Side:          side,
		Type:          orderType,
		Price:         limitPrice,
		Quantity:      quantity,
		Sequence:      seq,
	}
	envAccepted, err := messaging.NewEnvelope(messaging.SchemaOrderAccepted, "", acceptedPayload)
	if err != nil {
		return "", 0, fmt.Errorf("erro ao montar order.accepted: %w", err)
	}
	if err := s.publisher.Publish(ctx, messaging.ExchangeVeltraEvents, messaging.RKOrderAccepted, envAccepted, nil); err != nil {
		log.Printf("[Gateway] simulateTrade: falha ao publicar order.accepted: %v", err)
		return "", 0, fmt.Errorf("broker indisponivel (order.accepted): %w", err)
	}

	// 2. order.filled
	filledPayload := messaging.OrderFilledPayload{
		OrderID:          orderID,
		ClientOrderID:    clientOrderID,
		Account:          account,
		Pair:             pair,
		Side:             side,
		Price:            fillPrice,
		FillQuantity:     quantity,
		CumulativeFilled: quantity,
		RemainingQty:     0,
		Status:           "filled",
	}
	envFilled, err := messaging.NewEnvelope(messaging.SchemaOrderFilled, "", filledPayload)
	if err != nil {
		return "", 0, fmt.Errorf("erro ao montar order.filled: %w", err)
	}
	if err := s.publisher.Publish(ctx, messaging.ExchangeVeltraEvents, messaging.RKOrderFilled, envFilled, nil); err != nil {
		log.Printf("[Gateway] simulateTrade: falha ao publicar order.filled: %v", err)
		return "", 0, fmt.Errorf("broker indisponivel (order.filled): %w", err)
	}

	// 3. trade.executed
	tradePayload := messaging.TradeExecutedPayload{
		TradeID:      tradeIDStr,
		Pair:         pair,
		Price:        fillPrice,
		Quantity:     quantity,
		TakerOrderID: orderID,
		MakerOrderID: "_market",
		TakerAccount: account,
		MakerAccount: "_market",
		TakerSide:    side,
		Sequence:     seq,
		TimestampMs:  nowMs,
	}
	envTrade, err := messaging.NewEnvelope(messaging.SchemaTradeExecuted, "", tradePayload)
	if err != nil {
		return "", 0, fmt.Errorf("erro ao montar trade.executed: %w", err)
	}
	if err := s.publisher.Publish(ctx, messaging.ExchangeVeltraEvents, messaging.RKTradeExecuted, envTrade, nil); err != nil {
		log.Printf("[Gateway] simulateTrade: falha ao publicar trade.executed: %v", err)
		return "", 0, fmt.Errorf("broker indisponivel (trade.executed): %w", err)
	}

	return orderID, fillPrice, nil
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

	// ── OMS pré-trade: valida saldo disponível antes de qualquer execução ──
	if claims, ok := ClaimsFromContext(r.Context()); ok && s.auth != nil {
		baseAsset  := req.Pair[:strings.Index(req.Pair, "/")]
		quoteAsset := req.Pair[strings.Index(req.Pair, "/")+1:]

		var balErr error
		if req.Side == "sell" {
			// Venda: precisa ter o ativo base
			balErr = s.CheckSufficientBalance(r.Context(), claims.AccountID, baseAsset, int64(qty))
		} else {
			// Compra: precisa ter USDT-sim suficiente
			// Para market order, estima pelo preço atual de mercado
			var notional int64
			if req.Type == "limit" && int64(price) > 0 {
				// preço * quantidade / scale
				notional = int64(price) / 100_000_000 * int64(qty)
			} else {
				// market: usa preço de mercado
				mktPrice := s.veltra.MarketPriceFor(baseAsset)
				if mktPrice > 0 {
					notional = mktPrice / 100_000_000 * int64(qty)
				}
			}
			if notional > 0 {
				balErr = s.CheckSufficientBalance(r.Context(), claims.AccountID, quoteAsset, notional)
			}
		}
		if balErr != nil {
			writeError(w, http.StatusBadRequest, balErr.Error())
			return
		}
	}

	// Roteia: VLT/USDT-sim vai ao matching engine real; todos os outros pares
	// recebem fill simulado imediato no gateway.
	if req.Pair == "VLT/USDT-sim" {
		// Caminho original: publica order.place no exchange de comandos.
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
		if err := s.publisher.Publish(r.Context(), messaging.ExchangeVeltraCommands, messaging.RKOrderPlace, env, nil); err != nil {
			log.Printf("[Gateway] Falha ao publicar order.place: %v", err)
			writeError(w, http.StatusServiceUnavailable, "broker indisponivel")
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":          "queued",
			"tx_id":           env.TxID,
			"client_order_id": payload.ClientOrderID,
		})
		return
	}

	// Pares simulados: validar que o quote e USDT-sim.
	parsed, _ := exchange.ParsePair(req.Pair) // ja validado acima
	if string(parsed.Quote) != "USDT-sim" {
		writeError(w, http.StatusBadRequest, "apenas pares com quote USDT-sim sao suportados")
		return
	}

	// Garante client_order_id para a resposta.
	clientOrderID := req.ClientOrderID
	if clientOrderID == "" {
		clientOrderID = uuid.NewString()
	}

	orderID, fillPrice, err := s.simulateTrade(
		r.Context(),
		req.Account,
		req.Pair,
		req.Side,
		req.Type,
		int64(qty),
		int64(price),
		clientOrderID,
	)
	if err != nil {
		log.Printf("[Gateway] simulateTrade(%s): %v", req.Pair, err)
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "filled",
		"tx_id":           orderID,
		"client_order_id": clientOrderID,
		"fill_price":      fillPrice,
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
