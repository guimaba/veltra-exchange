package main

// veltra_state.go: projecoes (read models) da Veltra Exchange no gateway.
// Consome os eventos imutaveis do matching engine (via q.marketdata.events) e
// mantem em memoria: order book L2, trades recentes, ordens por conta e saldos.
//
// IMPORTANTE: os saldos aqui sao uma PROJECAO de exibicao (faucet.credit +
// trade.executed). A fonte de verdade contabil sera o ledger de dupla entrada
// (Fase 2, PostgreSQL); quando ele existir, esta projecao passa a ser apenas
// cache de leitura — exatamente o padrao CQRS do plano (secao 4.4).
//
// Todos os valores sao int64 na menor unidade (money.Scale). A conversao para
// decimal acontece apenas na borda (JSON -> UI).

import (
	"encoding/json"
	"sync"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// symbolHash retorna um valor deterministico baseado nos bytes do simbolo.
// Usado para gerar quantidades sinteticas do book sem dependencia de rand.
func symbolHash(sym string) int64 {
	var h int64 = 2166136261
	for i := 0; i < len(sym); i++ {
		h ^= int64(sym[i])
		h *= 16777619
	}
	if h < 0 {
		h = -h
	}
	return h
}

const (
	maxRecentTrades = 100
	maxOrderHistory = 200
)

// VeltraTrade e um trade na fita (formato de exibicao da projecao).
type VeltraTrade struct {
	TradeID     string `json:"trade_id"`
	Pair        string `json:"pair"`
	Price       int64  `json:"price"`
	Quantity    int64  `json:"quantity"`
	TakerSide   string `json:"taker_side"`
	TimestampMs int64  `json:"timestamp_ms"`
}

// VeltraOrder e o estado corrente de uma ordem na projecao.
type VeltraOrder struct {
	OrderID       string `json:"order_id"`
	ClientOrderID string `json:"client_order_id"`
	Account       string `json:"account"`
	Pair          string `json:"pair"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Price         int64  `json:"price"`
	Quantity      int64  `json:"quantity"`
	Filled        int64  `json:"filled"`
	Status        string `json:"status"`
	Sequence      uint64 `json:"sequence"`
}

// MarketCoin representa o preço em tempo real de uma criptomoeda (vem do cmd/marketdata).
type MarketCoin struct {
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	PriceUSD     float64 `json:"price_usd"`
	PriceBRL     float64 `json:"price_brl"`
	Change24h    float64 `json:"change_24h"`
	Volume24hUSD float64 `json:"volume_24h_usd"`
	MarketCapUSD float64 `json:"market_cap_usd"`
}

// Candle é uma vela OHLCV de 5 minutos.
type Candle struct {
	T int64   `json:"t"`
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
	V float64 `json:"v"`
}

// MarketUpdatePayload é o payload de market.update (publicado pelo cmd/marketdata).
type MarketUpdatePayload struct {
	Coins     []MarketCoin          `json:"coins"`
	Candles   map[string][]Candle   `json:"candles"`
	UpdatedAt int64                 `json:"updated_at"`
}

// VeltraState guarda as projecoes, protegidas por RWMutex (leituras da API
// REST concorrem com o consumer de eventos).
type VeltraState struct {
	mu sync.RWMutex

	books    map[string]messaging.BookUpdatedPayload // pair -> ultimo L2
	trades   map[string][]VeltraTrade                // pair -> fita (mais recente primeiro)
	orders   map[string]*VeltraOrder                 // orderID -> estado corrente
	orderSeq []string                                // ordem de chegada (para poda do historico)
	balances map[string]map[string]int64             // account -> asset -> saldo (menor unidade)
	lastPx   map[string]int64                        // pair -> preco do ultimo trade

	// market data (populado pelo cmd/marketdata via market.update)
	marketCoins     []MarketCoin
	marketCandles   map[string][]Candle // symbol -> candles
	marketUpdatedAt int64
}

func NewVeltraState() *VeltraState {
	return &VeltraState{
		books:         map[string]messaging.BookUpdatedPayload{},
		trades:        map[string][]VeltraTrade{},
		orders:        map[string]*VeltraOrder{},
		balances:      map[string]map[string]int64{},
		lastPx:        map[string]int64{},
		marketCandles: map[string][]Candle{},
	}
}

// ApplyEvent atualiza as projecoes a partir de um evento da Veltra.
func (s *VeltraState) ApplyEvent(routingKey string, env messaging.Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch routingKey {
	case messaging.RKBookUpdated:
		var p messaging.BookUpdatedPayload
		if env.Unmarshal(&p) == nil {
			// Descarta updates atrasados (sequence menor que o ja aplicado).
			if cur, ok := s.books[p.Pair]; !ok || p.Sequence >= cur.Sequence {
				s.books[p.Pair] = p
			}
		}

	case messaging.RKTradeExecuted:
		var p messaging.TradeExecutedPayload
		if env.Unmarshal(&p) == nil {
			s.applyTrade(p)
		}

	case messaging.RKFaucetCredit:
		var p messaging.FaucetCreditPayload
		if env.Unmarshal(&p) == nil && p.Account != "" && p.Amount > 0 {
			s.credit(p.Account, p.Asset, p.Amount)
		}

	case messaging.RKOrderAccepted:
		var p messaging.OrderAcceptedPayload
		if env.Unmarshal(&p) == nil {
			s.upsertOrder(&VeltraOrder{
				OrderID:       p.OrderID,
				ClientOrderID: p.ClientOrderID,
				Account:       p.Account,
				Pair:          p.Pair,
				Side:          p.Side,
				Type:          p.Type,
				Price:         p.Price,
				Quantity:      p.Quantity,
				Status:        "new",
				Sequence:      p.Sequence,
			})
		}

	case messaging.RKOrderFilled:
		var p messaging.OrderFilledPayload
		if env.Unmarshal(&p) == nil {
			if o, ok := s.orders[p.OrderID]; ok {
				o.Filled = p.CumulativeFilled
				o.Status = p.Status
			}
		}

	case messaging.RKOrderCanceled:
		var p messaging.OrderCanceledPayload
		if env.Unmarshal(&p) == nil {
			if o, ok := s.orders[p.OrderID]; ok {
				o.Status = "canceled"
			}
		}

	case messaging.RKOrderRejected:
		var p messaging.OrderRejectedPayload
		if env.Unmarshal(&p) == nil && p.ClientOrderID != "" {
			for _, o := range s.orders {
				if o.ClientOrderID == p.ClientOrderID {
					o.Status = "rejected"
				}
			}
		}

	case messaging.RKMarketUpdate:
		var p MarketUpdatePayload
		if err := json.Unmarshal(env.Payload, &p); err == nil {
			s.marketCoins = p.Coins
			s.marketUpdatedAt = p.UpdatedAt
			for sym, candles := range p.Candles {
				s.marketCandles[sym] = candles
			}
		}
	}
}

// MarketSnapshot retorna os dados de mercado para REST/WS.
func (s *VeltraState) MarketSnapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(map[string]any{
		"coins":      s.marketCoins,
		"updated_at": s.marketUpdatedAt,
	})
}

// CandlesForSymbol retorna as velas de um símbolo.
func (s *VeltraState) CandlesForSymbol(symbol string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	candles, ok := s.marketCandles[symbol]
	if !ok {
		return json.Marshal([]Candle{})
	}
	return json.Marshal(candles)
}

// applyTrade atualiza fita, ultimo preco e a projecao de saldos das duas pontas.
func (s *VeltraState) applyTrade(p messaging.TradeExecutedPayload) {
	t := VeltraTrade{
		TradeID:     p.TradeID,
		Pair:        p.Pair,
		Price:       p.Price,
		Quantity:    p.Quantity,
		TakerSide:   p.TakerSide,
		TimestampMs: p.TimestampMs,
	}
	tape := s.trades[p.Pair]
	tape = append([]VeltraTrade{t}, tape...)
	if len(tape) > maxRecentTrades {
		tape = tape[:maxRecentTrades]
	}
	s.trades[p.Pair] = tape
	s.lastPx[p.Pair] = p.Price

	// Movimentacao de saldos (projecao): comprador ganha BASE e paga QUOTE;
	// vendedor o inverso. Notional em aritmetica inteira (sem float).
	base, quote := splitPair(p.Pair)
	if base == "" {
		return
	}
	notional, err := money.Notional(money.Amount(p.Price), money.Amount(p.Quantity))
	if err != nil {
		return
	}
	buyer, seller := p.TakerAccount, p.MakerAccount
	if p.TakerSide == "sell" {
		buyer, seller = p.MakerAccount, p.TakerAccount
	}
	s.credit(buyer, base, p.Quantity)
	s.credit(buyer, quote, -int64(notional))
	s.credit(seller, base, -p.Quantity)
	s.credit(seller, quote, int64(notional))
}

func (s *VeltraState) credit(account, asset string, delta int64) {
	if s.balances[account] == nil {
		s.balances[account] = map[string]int64{}
	}
	s.balances[account][asset] += delta
}

func (s *VeltraState) upsertOrder(o *VeltraOrder) {
	if _, exists := s.orders[o.OrderID]; !exists {
		s.orderSeq = append(s.orderSeq, o.OrderID)
		// Poda do historico: remove as ordens mais antigas ja finalizadas.
		if len(s.orderSeq) > maxOrderHistory {
			keep := s.orderSeq[:0]
			removed := 0
			for _, id := range s.orderSeq {
				old := s.orders[id]
				if removed < len(s.orderSeq)-maxOrderHistory && old != nil &&
					(old.Status == "filled" || old.Status == "canceled" || old.Status == "rejected") {
					delete(s.orders, id)
					removed++
					continue
				}
				keep = append(keep, id)
			}
			s.orderSeq = keep
		}
	}
	s.orders[o.OrderID] = o
}

// SnapshotJSON serializa o estado da Veltra para o cliente (REST e WS inicial).
func (s *VeltraState) SnapshotJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	orders := make([]*VeltraOrder, 0, len(s.orderSeq))
	for i := len(s.orderSeq) - 1; i >= 0; i-- { // mais recente primeiro
		if o, ok := s.orders[s.orderSeq[i]]; ok {
			orders = append(orders, o)
		}
	}
	return json.Marshal(map[string]any{
		"books":      s.books,
		"trades":     s.trades,
		"orders":     orders,
		"balances":   s.balances,
		"last_price": s.lastPx,
	})
}

// MarketPriceFor retorna o preco corrente de um ativo (em menor unidade) a
// partir dos dados de market recebidos via market.update.
// Retorna 0 se o simbolo nao estiver na lista.
func (s *VeltraState) MarketPriceFor(symbol string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, c := range s.marketCoins {
		if c.Symbol == symbol {
			return int64(c.PriceUSD * float64(money.Scale))
		}
	}
	return 0
}

// SyntheticBook gera um book L2 sintetico de 5 niveis para pares simulados.
// Para VLT retorna nil (o book real vem do matching engine).
// Os niveis sao deterministicos dado o simbolo (sem rand), usando symbolHash.
func (s *VeltraState) SyntheticBook(symbol string) (bids, asks []messaging.BookLevel) {
	if symbol == "VLT" {
		return nil, nil
	}
	px := s.MarketPriceFor(symbol)
	if px == 0 {
		return nil, nil
	}

	h := symbolHash(symbol)
	// Quantidades base: entre 50_000_000 e 500_000_000 (scaled), variando por nivel.
	baseQty := int64(50_000_000) + (h%10)*int64(45_000_000) // 50M .. 455M

	askMults := [5]int64{1001, 1003, 1006, 1010, 1015} // +0.1%, +0.3%, +0.6%, +1.0%, +1.5%
	bidMults := [5]int64{999, 997, 994, 990, 985}       // -0.1%, -0.3%, -0.6%, -1.0%, -1.5%

	asks = make([]messaging.BookLevel, 5)
	bids = make([]messaging.BookLevel, 5)
	for i := 0; i < 5; i++ {
		// Quantidade decresce ligeiramente a cada nivel mais distante.
		qty := baseQty - int64(i)*int64(3_000_000+((h>>(uint(i)*3))%2_000_000))
		if qty < 50_000_000 {
			qty = 50_000_000
		}
		asks[i] = messaging.BookLevel{
			Price:    px * askMults[i] / 1000,
			Quantity: qty,
		}
		bids[i] = messaging.BookLevel{
			Price:    px * bidMults[i] / 1000,
			Quantity: qty,
		}
	}
	return bids, asks
}

// splitPair separa "BASE/QUOTE"; retorna "" se invalido.
func splitPair(pair string) (base, quote string) {
	for i := 0; i < len(pair); i++ {
		if pair[i] == '/' {
			return pair[:i], pair[i+1:]
		}
	}
	return "", ""
}
