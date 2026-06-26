package orderbook

import (
	"testing"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

var pair = exchange.Pair{Base: exchange.AssetVLT, Quote: exchange.AssetUSD}

func mkOrder(id string, side exchange.Side, price, qty string, seq uint64) *exchange.Order {
	return &exchange.Order{
		OrderID:  id,
		Account:  "acc-" + id,
		Pair:     pair,
		Side:     side,
		Type:     exchange.Limit,
		Price:    money.MustParse(price),
		Quantity: money.MustParse(qty),
		Status:   exchange.StatusNew,
		Sequence: seq,
	}
}

func TestPricePriority(t *testing.T) {
	ob := New(pair)
	// Bids: maior preco tem prioridade.
	ob.AddResting(mkOrder("b1", exchange.Buy, "100", "1", 1))
	ob.AddResting(mkOrder("b2", exchange.Buy, "101", "1", 2))
	ob.AddResting(mkOrder("b3", exchange.Buy, "99", "1", 3))
	if front := ob.Front(exchange.Buy); front.OrderID != "b2" {
		t.Errorf("melhor bid deveria ser b2 (101), veio %s", front.OrderID)
	}

	// Asks: menor preco tem prioridade.
	ob.AddResting(mkOrder("a1", exchange.Sell, "105", "1", 4))
	ob.AddResting(mkOrder("a2", exchange.Sell, "103", "1", 5))
	if front := ob.Front(exchange.Sell); front.OrderID != "a2" {
		t.Errorf("melhor ask deveria ser a2 (103), veio %s", front.OrderID)
	}
}

func TestTimePriorityFIFO(t *testing.T) {
	ob := New(pair)
	// Mesmo preco: a que chegou antes (menor Sequence) sai primeiro.
	ob.AddResting(mkOrder("first", exchange.Buy, "100", "1", 1))
	ob.AddResting(mkOrder("second", exchange.Buy, "100", "1", 2))
	ob.AddResting(mkOrder("third", exchange.Buy, "100", "1", 3))

	order := []string{"first", "second", "third"}
	for _, want := range order {
		got := ob.Front(exchange.Buy)
		if got.OrderID != want {
			t.Fatalf("FIFO quebrado: esperava %s, veio %s", want, got.OrderID)
		}
		ob.PopFront(exchange.Buy)
	}
	if ob.Front(exchange.Buy) != nil {
		t.Error("book deveria estar vazio")
	}
}

func TestCancel(t *testing.T) {
	ob := New(pair)
	ob.AddResting(mkOrder("b1", exchange.Buy, "100", "1", 1))
	ob.AddResting(mkOrder("b2", exchange.Buy, "100", "1", 2))

	o, ok := ob.Cancel("b1")
	if !ok || o.OrderID != "b1" {
		t.Fatalf("cancel de b1 falhou: ok=%v", ok)
	}
	if _, ok := ob.Cancel("b1"); ok {
		t.Error("cancel duplo deveria falhar")
	}
	if front := ob.Front(exchange.Buy); front == nil || front.OrderID != "b2" {
		t.Error("apos cancelar b1, front deveria ser b2")
	}
	if ob.Len() != 1 {
		t.Errorf("Len deveria ser 1, veio %d", ob.Len())
	}
}

func TestBestAndSpread(t *testing.T) {
	ob := New(pair)
	if _, ok := ob.BestBid(); ok {
		t.Error("book vazio nao deveria ter best bid")
	}
	ob.AddResting(mkOrder("b1", exchange.Buy, "99", "1", 1))
	ob.AddResting(mkOrder("a1", exchange.Sell, "101", "1", 2))

	bid, _ := ob.BestBid()
	ask, _ := ob.BestAsk()
	if bid != money.MustParse("99") || ask != money.MustParse("101") {
		t.Errorf("best bid/ask incorretos: %s / %s", bid, ask)
	}
	spread, ok := ob.Spread()
	if !ok || spread != money.MustParse("2") {
		t.Errorf("spread deveria ser 2, veio %s", spread)
	}
}

func TestL2Aggregation(t *testing.T) {
	ob := New(pair)
	// Dois bids no mesmo preco -> agregam quantidade.
	ob.AddResting(mkOrder("b1", exchange.Buy, "100", "1.5", 1))
	ob.AddResting(mkOrder("b2", exchange.Buy, "100", "2.5", 2))
	ob.AddResting(mkOrder("b3", exchange.Buy, "99", "1", 3))

	bids, _ := ob.L2(0)
	if len(bids) != 2 {
		t.Fatalf("esperava 2 niveis de bid, veio %d", len(bids))
	}
	if bids[0].Price != money.MustParse("100") || bids[0].Quantity != money.MustParse("4") {
		t.Errorf("nivel 100 deveria agregar 4.0, veio %s @ %s", bids[0].Quantity, bids[0].Price)
	}
	if bids[1].Price != money.MustParse("99") {
		t.Errorf("segundo nivel deveria ser 99, veio %s", bids[1].Price)
	}

	// depth limita
	bidsTop, _ := ob.L2(1)
	if len(bidsTop) != 1 {
		t.Errorf("depth=1 deveria retornar 1 nivel, veio %d", len(bidsTop))
	}
}
