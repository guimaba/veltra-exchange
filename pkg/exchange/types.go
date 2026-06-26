// Package exchange define os tipos de dominio da Veltra Exchange: ativos, pares
// de negociacao, ordens e trades. Todos os valores monetarios usam money.Amount
// (inteiro escalado), nunca float64, conforme o plano tecnico (secao 4.2.2).
//
// Convencao de precificacao (estilo CLOB spot):
//   - Em um par BASE/QUOTE (ex.: BTC/USD), a quantidade (Quantity) e medida
//     em unidades do ativo BASE, e o preco (Price) em unidades de QUOTE por 1 BASE.
//   - Comprar (Buy) gasta QUOTE e recebe BASE; vender (Sell) gasta BASE e recebe
//     QUOTE.
//   - As moedas de cotacao (QUOTE) sao as fiat USD, BRL e EUR: cada cripto
//     negocia diretamente contra as tres (ex.: BTC/USD, BTC/BRL, BTC/EUR).
package exchange

import (
	"fmt"
	"strings"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// Asset e o codigo de um ativo (ex.: "VLT", "BTC", "USD").
type Asset string

// Ativos nativos do ambiente simulado.
const (
	AssetVLT Asset = "VLT" // token nativo ficticio
	AssetUSD Asset = "USD" // dolar americano (moeda de cotacao)
	AssetBRL Asset = "BRL" // real brasileiro (moeda de cotacao)
	AssetEUR Asset = "EUR" // euro (moeda de cotacao)
)

// QuoteAssets sao as moedas fiat aceitas como cotacao (lado QUOTE de um par).
var QuoteAssets = []Asset{AssetUSD, AssetBRL, AssetEUR}

// IsQuoteAsset informa se o ativo pode ser usado como moeda de cotacao.
func IsQuoteAsset(a Asset) bool {
	for _, q := range QuoteAssets {
		if a == q {
			return true
		}
	}
	return false
}

// Pair representa um par de negociacao BASE/QUOTE.
type Pair struct {
	Base  Asset `json:"base"`
	Quote Asset `json:"quote"`
}

// String retorna o simbolo no formato "BASE/QUOTE" (ex.: "BTC/USD").
func (p Pair) String() string {
	return string(p.Base) + "/" + string(p.Quote)
}

// ParsePair interpreta "BASE/QUOTE" em um Pair.
func ParsePair(s string) (Pair, error) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Pair{}, fmt.Errorf("par invalido: %q", s)
	}
	return Pair{Base: Asset(parts[0]), Quote: Asset(parts[1])}, nil
}

// Side indica o lado da ordem.
type Side string

const (
	Buy  Side = "buy"
	Sell Side = "sell"
)

// Opposite retorna o lado oposto (usado para achar a contraparte no book).
func (s Side) Opposite() Side {
	if s == Buy {
		return Sell
	}
	return Buy
}

// OrderType define o tipo de execucao da ordem. limit e market sao o MVP;
// stop-loss/stop-limit/OCO ficam para a fase avancada (plano secao 4.2.3).
type OrderType string

const (
	Limit     OrderType = "limit"
	Market    OrderType = "market"
	StopLoss  OrderType = "stop_loss"  // fase avancada
	StopLimit OrderType = "stop_limit" // fase avancada
)

// TimeInForce controla por quanto tempo uma ordem permanece ativa.
type TimeInForce string

const (
	GTC TimeInForce = "gtc" // Good-Til-Canceled: repousa no book ate casar/cancelar
	IOC TimeInForce = "ioc" // Immediate-Or-Cancel: casa o que der, cancela o resto
	FOK TimeInForce = "fok" // Fill-Or-Kill: casa tudo ou nada
)

// OrderStatus e o estado do ciclo de vida de uma ordem.
type OrderStatus string

const (
	StatusNew             OrderStatus = "new"
	StatusPartiallyFilled OrderStatus = "partially_filled"
	StatusFilled          OrderStatus = "filled"
	StatusCanceled        OrderStatus = "canceled"
	StatusRejected        OrderStatus = "rejected"
)

// Order e uma ordem submetida ao matching engine.
//
// Sequence e o numero de sequencia global atribuido pelo sequenciador
// deterministico ANTES do matching. A prioridade preco-tempo (FIFO) usa
// Sequence — e nao o relogio de parede — para garantir determinismo total no
// replay do log de eventos (plano secao 4.2.2).
type Order struct {
	OrderID       string       `json:"order_id"`        // id interno, unico
	ClientOrderID string       `json:"client_order_id"` // idempotencia por cliente
	Account       string       `json:"account"`
	Pair          Pair         `json:"pair"`
	Side          Side         `json:"side"`
	Type          OrderType    `json:"type"`
	TimeInForce   TimeInForce  `json:"time_in_force"`
	Price         money.Amount `json:"price"`    // 0 para ordens market
	Quantity      money.Amount `json:"quantity"` // quantidade original em BASE
	Filled        money.Amount `json:"filled"`   // quantidade ja executada em BASE
	Status        OrderStatus  `json:"status"`
	Sequence      uint64       `json:"sequence"` // ordem global deterministica
	TimestampMs   int64        `json:"timestamp_ms"`
}

// Remaining retorna a quantidade ainda nao executada (Quantity - Filled).
func (o *Order) Remaining() money.Amount {
	r, _ := o.Quantity.Sub(o.Filled)
	return r
}

// IsResting indica se a ordem pode repousar no book (limit GTC nao finalizada).
func (o *Order) IsResting() bool {
	return o.Status == StatusNew || o.Status == StatusPartiallyFilled
}

// Trade e o resultado de um casamento entre uma ordem agressora (taker) e uma
// ordem em repouso (maker). E o evento imutavel (trade.executed) que alimenta o
// ledger e o market data.
type Trade struct {
	TradeID      string       `json:"trade_id"`
	Pair         Pair         `json:"pair"`
	Price        money.Amount `json:"price"`    // preco de execucao (preco do maker)
	Quantity     money.Amount `json:"quantity"` // quantidade casada em BASE
	TakerOrderID string       `json:"taker_order_id"`
	MakerOrderID string       `json:"maker_order_id"`
	TakerAccount string       `json:"taker_account"`
	MakerAccount string       `json:"maker_account"`
	TakerSide    Side         `json:"taker_side"`
	Sequence     uint64       `json:"sequence"` // sequencia do trade no log do par
	TimestampMs  int64        `json:"timestamp_ms"`
}

// BuyerSeller resolve qual conta compra e qual vende a partir do lado do taker.
// Util para o settlement de dupla entrada no ledger.
func (t *Trade) BuyerSeller() (buyer, seller string) {
	if t.TakerSide == Buy {
		return t.TakerAccount, t.MakerAccount
	}
	return t.MakerAccount, t.TakerAccount
}
