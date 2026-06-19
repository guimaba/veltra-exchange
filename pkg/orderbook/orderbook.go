// Package orderbook implementa um Central Limit Order Book (CLOB) com prioridade
// preco-tempo (FIFO), conforme o plano tecnico (secao 4.2.1):
//
//   - Preco primeiro: o melhor preco tem prioridade — maior bid para
//     compradores, menor ask para vendedores.
//   - Tempo em segundo: no mesmo preco, a ordem que chegou antes (menor
//     Sequence) e preenchida primeiro.
//
// A estrutura NAO e thread-safe por design: o matching engine a opera em uma
// unica goroutine por par (single-threaded, secao 4.2.2), eliminando condicoes
// de corrida e garantindo determinismo.
package orderbook

import (
	"sort"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// Level e um nivel agregado do book (preco -> quantidade total em repouso).
type Level struct {
	Price    money.Amount
	Quantity money.Amount
}

// priceLevel guarda as ordens em repouso de um mesmo preco, em ordem FIFO
// (a primeira da fatia foi a primeira a chegar).
type priceLevel struct {
	price  money.Amount
	orders []*exchange.Order
}

// totalQty soma a quantidade remanescente de todas as ordens do nivel.
func (pl *priceLevel) totalQty() money.Amount {
	var sum money.Amount
	for _, o := range pl.orders {
		sum += o.Remaining()
	}
	return sum
}

// bookSide e um lado do book (bids ou asks) com niveis ordenados por prioridade
// de preco: bids em ordem decrescente, asks em ordem crescente. Assim levels[0]
// e sempre o melhor preco do lado.
type bookSide struct {
	isBid  bool
	levels []*priceLevel
}

// locate acha o indice do nivel de um preco. found indica se ja existe; caso
// contrario idx e a posicao de insercao que preserva a ordenacao.
func (s *bookSide) locate(price money.Amount) (idx int, found bool) {
	idx = sort.Search(len(s.levels), func(i int) bool {
		if s.isBid {
			return s.levels[i].price <= price // ordem decrescente
		}
		return s.levels[i].price >= price // ordem crescente
	})
	if idx < len(s.levels) && s.levels[idx].price == price {
		return idx, true
	}
	return idx, false
}

func (s *bookSide) add(o *exchange.Order) {
	idx, found := s.locate(o.Price)
	if found {
		s.levels[idx].orders = append(s.levels[idx].orders, o)
		return
	}
	lv := &priceLevel{price: o.Price, orders: []*exchange.Order{o}}
	s.levels = append(s.levels, nil)
	copy(s.levels[idx+1:], s.levels[idx:])
	s.levels[idx] = lv
}

// front retorna a ordem de maior prioridade (melhor preco, mais antiga) ou nil.
func (s *bookSide) front() *exchange.Order {
	if len(s.levels) == 0 {
		return nil
	}
	return s.levels[0].orders[0]
}

// popFront remove a ordem de maior prioridade, descartando o nivel se ficar vazio.
func (s *bookSide) popFront() {
	if len(s.levels) == 0 {
		return
	}
	lv := s.levels[0]
	lv.orders[0] = nil
	lv.orders = lv.orders[1:]
	if len(lv.orders) == 0 {
		s.levels[0] = nil
		s.levels = s.levels[1:]
	}
}

// remove tira uma ordem especifica do lado (cancelamento). Retorna true se achou.
func (s *bookSide) remove(orderID string, price money.Amount) bool {
	idx, found := s.locate(price)
	if !found {
		return false
	}
	lv := s.levels[idx]
	for i, o := range lv.orders {
		if o.OrderID == orderID {
			lv.orders = append(lv.orders[:i], lv.orders[i+1:]...)
			if len(lv.orders) == 0 {
				s.levels = append(s.levels[:idx], s.levels[idx+1:]...)
			}
			return true
		}
	}
	return false
}

// OrderBook e o CLOB de um par. Veja a doc do pacote sobre thread-safety.
type OrderBook struct {
	Pair  exchange.Pair
	bids  *bookSide
	asks  *bookSide
	index map[string]*exchange.Order // orderID -> ordem em repouso (para cancelamento)
}

// New cria um book vazio para o par dado.
func New(pair exchange.Pair) *OrderBook {
	return &OrderBook{
		Pair:  pair,
		bids:  &bookSide{isBid: true},
		asks:  &bookSide{isBid: false},
		index: make(map[string]*exchange.Order),
	}
}

func (ob *OrderBook) sideFor(s exchange.Side) *bookSide {
	if s == exchange.Buy {
		return ob.bids
	}
	return ob.asks
}

// AddResting insere uma ordem (limit, parcialmente preenchida ou nao) no book.
func (ob *OrderBook) AddResting(o *exchange.Order) {
	ob.sideFor(o.Side).add(o)
	ob.index[o.OrderID] = o
}

// Front retorna a ordem de maior prioridade do lado indicado, ou nil se vazio.
// Para casar uma ordem de compra, consulte Front(Sell) (o melhor ask).
func (ob *OrderBook) Front(s exchange.Side) *exchange.Order {
	return ob.sideFor(s).front()
}

// PopFront remove a ordem de maior prioridade do lado (use quando totalmente
// preenchida).
func (ob *OrderBook) PopFront(s exchange.Side) {
	if o := ob.sideFor(s).front(); o != nil {
		delete(ob.index, o.OrderID)
	}
	ob.sideFor(s).popFront()
}

// Cancel remove uma ordem em repouso pelo OrderID. Retorna a ordem e true se
// encontrada.
func (ob *OrderBook) Cancel(orderID string) (*exchange.Order, bool) {
	o, ok := ob.index[orderID]
	if !ok {
		return nil, false
	}
	if ob.sideFor(o.Side).remove(orderID, o.Price) {
		delete(ob.index, orderID)
		return o, true
	}
	return nil, false
}

// Get retorna uma ordem em repouso pelo id (sem remover).
func (ob *OrderBook) Get(orderID string) (*exchange.Order, bool) {
	o, ok := ob.index[orderID]
	return o, ok
}

// BestBid / BestAsk retornam o melhor preco do lado e true se houver ordens.
func (ob *OrderBook) BestBid() (money.Amount, bool) { return ob.best(ob.bids) }
func (ob *OrderBook) BestAsk() (money.Amount, bool) { return ob.best(ob.asks) }

func (ob *OrderBook) best(s *bookSide) (money.Amount, bool) {
	if len(s.levels) == 0 {
		return 0, false
	}
	return s.levels[0].price, true
}

// Spread retorna (ask - bid) quando ambos os lados tem ordens.
func (ob *OrderBook) Spread() (money.Amount, bool) {
	bid, okB := ob.BestBid()
	ask, okA := ob.BestAsk()
	if !okB || !okA {
		return 0, false
	}
	return ask - bid, true
}

// L2 retorna ate `depth` niveis agregados de cada lado, em ordem de prioridade
// (melhor preco primeiro). depth <= 0 retorna todos os niveis.
func (ob *OrderBook) L2(depth int) (bids, asks []Level) {
	return ob.levels(ob.bids, depth), ob.levels(ob.asks, depth)
}

func (ob *OrderBook) levels(s *bookSide, depth int) []Level {
	n := len(s.levels)
	if depth > 0 && depth < n {
		n = depth
	}
	out := make([]Level, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Level{Price: s.levels[i].price, Quantity: s.levels[i].totalQty()})
	}
	return out
}

// Len retorna o numero total de ordens em repouso no book.
func (ob *OrderBook) Len() int {
	return len(ob.index)
}

// AllOrders retorna todas as ordens em repouso em ordem deterministica de
// prioridade: primeiro os bids (melhor preco, FIFO), depois os asks. Reinserir
// o resultado via AddResting em um book vazio reconstroi exatamente o mesmo
// estado — usado por snapshots (plano secao 4.2.2).
func (ob *OrderBook) AllOrders() []*exchange.Order {
	out := make([]*exchange.Order, 0, len(ob.index))
	for _, s := range []*bookSide{ob.bids, ob.asks} {
		for _, lv := range s.levels {
			out = append(out, lv.orders...)
		}
	}
	return out
}
