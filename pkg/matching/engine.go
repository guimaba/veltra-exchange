// Package matching implementa o motor de casamento (matching engine) da Veltra
// Exchange: um CLOB com prioridade preco-tempo, operado single-threaded por par
// de negociacao (plano tecnico secao 4.2).
//
// Determinismo (secao 4.2.2): o motor nunca consulta relogio de parede nem
// gera aleatoriedade. Toda ordem chega ja sequenciada (Order.Sequence) e com
// timestamp (Order.TimestampMs); IDs de trade sao derivados da sequencia. Logo,
// o mesmo log de entrada sempre produz exatamente o mesmo resultado — base do
// replay via WAL/snapshots.
package matching

import (
	"fmt"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/orderbook"
)

// Fill e o snapshot de uma execucao aplicada a uma ordem (execution report).
// Mapeia diretamente para messaging.OrderFilledPayload.
type Fill struct {
	OrderID          string
	ClientOrderID    string
	Account          string
	Side             exchange.Side
	Price            money.Amount // preco do fill (preco do maker)
	FillQty          money.Amount
	CumulativeFilled money.Amount
	Remaining        money.Amount
	Status           exchange.OrderStatus
}

// Result agrega tudo que um Process produz: trades, execution reports, ordens
// canceladas por self-trade prevention, e o estado final da ordem agressora.
type Result struct {
	Trades         []exchange.Trade
	Fills          []Fill            // reports do taker e dos makers
	CanceledMakers []*exchange.Order // makers cancelados por self-trade prevention
	Rested         bool              // taker repousou no book
	TakerStatus    exchange.OrderStatus
	RejectReason   string // preenchido quando TakerStatus == StatusRejected
}

// Engine e o motor de um unico par. NAO e thread-safe: deve ser acessado por
// uma unica goroutine (o loop sequenciado do servico).
type Engine struct {
	Pair     exchange.Pair
	book     *orderbook.OrderBook
	tradeSeq uint64 // contador de trades do par (deterministico)
}

// NewEngine cria um motor vazio para o par.
func NewEngine(pair exchange.Pair) *Engine {
	return &Engine{Pair: pair, book: orderbook.New(pair)}
}

// Book expoe o order book (leitura) para projecoes de market data e snapshots.
func (e *Engine) Book() *orderbook.OrderBook { return e.book }

// TradeSeq retorna o contador atual de trades (para snapshots/restauracao).
func (e *Engine) TradeSeq() uint64 { return e.tradeSeq }

// SetTradeSeq restaura o contador de trades (usado ao recarregar de snapshot).
func (e *Engine) SetTradeSeq(s uint64) { e.tradeSeq = s }

// Process executa o casamento de uma ordem agressora (taker) ja sequenciada.
// A ordem e mutada (Filled/Status). Retorna trades e execution reports.
func (e *Engine) Process(taker *exchange.Order) Result {
	if reason := e.validate(taker); reason != "" {
		taker.Status = exchange.StatusRejected
		return Result{TakerStatus: exchange.StatusRejected, RejectReason: reason}
	}

	// FOK: so executa se conseguir preencher tudo de imediato.
	if taker.TimeInForce == exchange.FOK && !e.canFullyFill(taker) {
		taker.Status = exchange.StatusRejected
		return Result{TakerStatus: exchange.StatusRejected, RejectReason: "FOK_NOT_FULLY_FILLABLE"}
	}

	var res Result
	opposite := taker.Side.Opposite()

	for taker.Remaining().IsPositive() {
		maker := e.book.Front(opposite)
		if maker == nil {
			break // book esgotado deste lado
		}
		if !e.crosses(taker, maker) {
			break // melhor preco da contraparte nao cruza
		}

		// Self-trade prevention (secao 4.2.2 / 5.2): impede que ordens da mesma
		// conta casem entre si. Politica: cancela a ordem em repouso (maker) e
		// segue para a proxima.
		if maker.Account == taker.Account {
			e.book.PopFront(opposite)
			maker.Status = exchange.StatusCanceled
			res.CanceledMakers = append(res.CanceledMakers, maker)
			continue
		}

		qty := minAmount(taker.Remaining(), maker.Remaining())
		price := maker.Price // execucao ao preco do maker (price improvement ao taker)

		// Aplica os fills.
		taker.Filled += qty
		maker.Filled += qty
		updateStatus(taker)
		updateStatus(maker)

		e.tradeSeq++
		trade := exchange.Trade{
			TradeID:      fmt.Sprintf("%s-t%d", e.Pair.String(), e.tradeSeq),
			Pair:         e.Pair,
			Price:        price,
			Quantity:     qty,
			TakerOrderID: taker.OrderID,
			MakerOrderID: maker.OrderID,
			TakerAccount: taker.Account,
			MakerAccount: maker.Account,
			TakerSide:    taker.Side,
			Sequence:     e.tradeSeq,
			TimestampMs:  taker.TimestampMs, // herdado: determinismo
		}
		res.Trades = append(res.Trades, trade)
		res.Fills = append(res.Fills, fillOf(taker, price, qty), fillOf(maker, price, qty))

		if maker.Remaining().IsZero() {
			e.book.PopFront(opposite)
		}
	}

	e.finalize(taker, &res)
	return res
}

// RestingOrders retorna as ordens em repouso em ordem de prioridade (para
// snapshots).
func (e *Engine) RestingOrders() []*exchange.Order {
	return e.book.AllOrders()
}

// Restore reconstroi o estado do motor a partir de um snapshot: reinsere as
// ordens em repouso (na ordem dada) e restaura o contador de trades.
func (e *Engine) Restore(orders []*exchange.Order, tradeSeq uint64) {
	e.book = orderbook.New(e.Pair)
	for _, o := range orders {
		e.book.AddResting(o)
	}
	e.tradeSeq = tradeSeq
}

// Cancel remove uma ordem em repouso. Retorna a ordem cancelada e true se achou.
func (e *Engine) Cancel(orderID string) (*exchange.Order, bool) {
	o, ok := e.book.Cancel(orderID)
	if ok {
		o.Status = exchange.StatusCanceled
	}
	return o, ok
}

// finalize decide o destino da quantidade remanescente do taker conforme tipo e
// time-in-force.
func (e *Engine) finalize(taker *exchange.Order, res *Result) {
	if taker.Remaining().IsZero() {
		taker.Status = exchange.StatusFilled
		res.TakerStatus = exchange.StatusFilled
		return
	}

	// Market e IOC nao repousam: o que sobrou e cancelado.
	if taker.Type == exchange.Market || taker.TimeInForce == exchange.IOC {
		if taker.Filled.IsZero() {
			taker.Status = exchange.StatusCanceled
		} else {
			taker.Status = exchange.StatusPartiallyFilled
		}
		res.TakerStatus = taker.Status
		res.Rested = false
		return
	}

	// Limit GTC: repousa o remanescente no book.
	updateStatus(taker)
	e.book.AddResting(taker)
	res.TakerStatus = taker.Status
	res.Rested = true
}

// validate retorna uma razao de rejeicao, ou "" se a ordem e valida.
func (e *Engine) validate(o *exchange.Order) string {
	if o.Pair != e.Pair {
		return "WRONG_PAIR"
	}
	if !o.Quantity.IsPositive() {
		return "INVALID_QUANTITY"
	}
	switch o.Type {
	case exchange.Limit:
		if !o.Price.IsPositive() {
			return "INVALID_PRICE"
		}
	case exchange.Market:
		// market ignora preco
	default:
		return "UNSUPPORTED_ORDER_TYPE"
	}
	if o.Side != exchange.Buy && o.Side != exchange.Sell {
		return "INVALID_SIDE"
	}
	return ""
}

// crosses indica se a ordem agressora cruza com o preco do maker.
func (e *Engine) crosses(taker, maker *exchange.Order) bool {
	if taker.Type == exchange.Market {
		return true // market cruza a qualquer preco disponivel
	}
	if taker.Side == exchange.Buy {
		return taker.Price.Cmp(maker.Price) >= 0 // compra paga ate Price
	}
	return taker.Price.Cmp(maker.Price) <= 0 // venda aceita a partir de Price
}

// canFullyFill verifica (sem mutar o book) se ha liquidez para preencher toda a
// quantidade do taker aos precos que cruzam. Usado por FOK.
func (e *Engine) canFullyFill(taker *exchange.Order) bool {
	need := taker.Quantity
	opposite := taker.Side.Opposite()
	var bids, asks []orderbook.Level
	b, a := e.book.L2(0)
	bids, asks = b, a

	levels := asks
	if opposite == exchange.Buy {
		levels = bids
	}
	var avail money.Amount
	for _, lv := range levels {
		// respeita o limite de preco (market ignora)
		if taker.Type != exchange.Market {
			if taker.Side == exchange.Buy && taker.Price.Cmp(lv.Price) < 0 {
				break
			}
			if taker.Side == exchange.Sell && taker.Price.Cmp(lv.Price) > 0 {
				break
			}
		}
		avail += lv.Quantity
		if avail.Cmp(need) >= 0 {
			return true
		}
	}
	return false
}

// ----- helpers -----

func fillOf(o *exchange.Order, price, qty money.Amount) Fill {
	return Fill{
		OrderID:          o.OrderID,
		ClientOrderID:    o.ClientOrderID,
		Account:          o.Account,
		Side:             o.Side,
		Price:            price,
		FillQty:          qty,
		CumulativeFilled: o.Filled,
		Remaining:        o.Remaining(),
		Status:           o.Status,
	}
}

func updateStatus(o *exchange.Order) {
	switch {
	case o.Remaining().IsZero():
		o.Status = exchange.StatusFilled
	case o.Filled.IsPositive():
		o.Status = exchange.StatusPartiallyFilled
	default:
		o.Status = exchange.StatusNew
	}
}

func minAmount(a, b money.Amount) money.Amount {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}
