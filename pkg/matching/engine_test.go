package matching

import (
	"testing"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

var pair = exchange.Pair{Base: exchange.AssetVLT, Quote: exchange.AssetUSD}

func ord(id, acc string, side exchange.Side, typ exchange.OrderType, tif exchange.TimeInForce, price, qty string, seq uint64) *exchange.Order {
	o := &exchange.Order{
		OrderID:     id,
		Account:     acc,
		Pair:        pair,
		Side:        side,
		Type:        typ,
		TimeInForce: tif,
		Quantity:    money.MustParse(qty),
		Status:      exchange.StatusNew,
		Sequence:    seq,
		TimestampMs: int64(seq) * 1000, // deterministico
	}
	if price != "" {
		o.Price = money.MustParse(price)
	}
	return o
}

func TestNoCrossRests(t *testing.T) {
	e := NewEngine(pair)
	// Bid abaixo do ask: nao casa, ambos repousam.
	r1 := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "99", "1", 1))
	if len(r1.Trades) != 0 || !r1.Rested {
		t.Fatalf("bid isolado nao deveria casar; rested=%v trades=%d", r1.Rested, len(r1.Trades))
	}
	r2 := e.Process(ord("a1", "bob", exchange.Sell, exchange.Limit, exchange.GTC, "101", "1", 2))
	if len(r2.Trades) != 0 || !r2.Rested {
		t.Fatalf("ask isolado nao deveria casar")
	}
	bid, _ := e.Book().BestBid()
	ask, _ := e.Book().BestAsk()
	if bid != money.MustParse("99") || ask != money.MustParse("101") {
		t.Errorf("book inesperado: %s / %s", bid, ask)
	}
}

func TestFullFill(t *testing.T) {
	e := NewEngine(pair)
	e.Process(ord("a1", "bob", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 1))
	r := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "100", "1", 2))

	if len(r.Trades) != 1 {
		t.Fatalf("esperava 1 trade, veio %d", len(r.Trades))
	}
	tr := r.Trades[0]
	if tr.Price != money.MustParse("100") || tr.Quantity != money.MustParse("1") {
		t.Errorf("trade incorreto: %s @ %s", tr.Quantity, tr.Price)
	}
	if tr.TakerAccount != "alice" || tr.MakerAccount != "bob" {
		t.Errorf("partes incorretas: taker=%s maker=%s", tr.TakerAccount, tr.MakerAccount)
	}
	if r.TakerStatus != exchange.StatusFilled || r.Rested {
		t.Errorf("taker deveria estar filled e nao repousar")
	}
	if e.Book().Len() != 0 {
		t.Errorf("book deveria ficar vazio, tem %d", e.Book().Len())
	}
}

func TestPartialFillRestsRemainder(t *testing.T) {
	e := NewEngine(pair)
	e.Process(ord("a1", "bob", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 1))
	// Compra 3 @ 100, mas so ha 1 disponivel: casa 1, repousa 2.
	r := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "100", "3", 2))

	if len(r.Trades) != 1 || r.Trades[0].Quantity != money.MustParse("1") {
		t.Fatalf("esperava 1 trade de 1.0")
	}
	if r.TakerStatus != exchange.StatusPartiallyFilled || !r.Rested {
		t.Errorf("taker deveria estar parcial e repousando, veio %s rested=%v", r.TakerStatus, r.Rested)
	}
	bid, _ := e.Book().BestBid()
	if bid != money.MustParse("100") {
		t.Errorf("remanescente deveria repousar como bid 100")
	}
}

func TestPriceTimePriority(t *testing.T) {
	e := NewEngine(pair)
	// Tres asks: dois a 100 (FIFO) e um a 101. Uma compra grande deve consumir
	// 100(a1), depois 100(a2), depois 101(a3).
	e.Process(ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 1))
	e.Process(ord("a2", "s2", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 2))
	e.Process(ord("a3", "s3", exchange.Sell, exchange.Limit, exchange.GTC, "101", "1", 3))

	r := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "101", "3", 4))
	if len(r.Trades) != 3 {
		t.Fatalf("esperava 3 trades, veio %d", len(r.Trades))
	}
	wantMakers := []string{"a1", "a2", "a3"}
	wantPrices := []string{"100", "100", "101"}
	for i, tr := range r.Trades {
		if tr.MakerOrderID != wantMakers[i] {
			t.Errorf("trade %d: maker %s, quero %s", i, tr.MakerOrderID, wantMakers[i])
		}
		if tr.Price != money.MustParse(wantPrices[i]) {
			t.Errorf("trade %d: preco %s, quero %s", i, tr.Price, wantPrices[i])
		}
	}
}

func TestMarketOrderSweepsBook(t *testing.T) {
	e := NewEngine(pair)
	e.Process(ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 1))
	e.Process(ord("a2", "s2", exchange.Sell, exchange.Limit, exchange.GTC, "105", "1", 2))

	// Market buy de 2: cruza ambos os niveis independente do preco.
	r := e.Process(ord("m1", "alice", exchange.Buy, exchange.Market, exchange.IOC, "", "2", 3))
	if len(r.Trades) != 2 {
		t.Fatalf("market deveria varrer 2 niveis, veio %d", len(r.Trades))
	}
	if r.TakerStatus != exchange.StatusFilled {
		t.Errorf("market totalmente preenchida deveria estar filled, veio %s", r.TakerStatus)
	}
}

func TestMarketOrderNoLiquidityCanceled(t *testing.T) {
	e := NewEngine(pair)
	r := e.Process(ord("m1", "alice", exchange.Buy, exchange.Market, exchange.IOC, "", "1", 1))
	if len(r.Trades) != 0 || r.TakerStatus != exchange.StatusCanceled || r.Rested {
		t.Errorf("market sem liquidez deveria ser cancelada sem repousar")
	}
}

func TestIOCCancelsRemainder(t *testing.T) {
	e := NewEngine(pair)
	e.Process(ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 1))
	r := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.IOC, "100", "3", 2))
	if len(r.Trades) != 1 || r.Rested {
		t.Fatalf("IOC deveria casar 1 e nao repousar")
	}
	if r.TakerStatus != exchange.StatusPartiallyFilled {
		t.Errorf("IOC parcial deveria reportar partially_filled, veio %s", r.TakerStatus)
	}
}

func TestFOKRejectedWhenInsufficient(t *testing.T) {
	e := NewEngine(pair)
	e.Process(ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 1))
	r := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.FOK, "100", "3", 2))
	if r.TakerStatus != exchange.StatusRejected || len(r.Trades) != 0 {
		t.Fatalf("FOK sem liquidez total deveria rejeitar sem trades, veio %s trades=%d", r.TakerStatus, len(r.Trades))
	}
	// Garante que nada foi consumido do book.
	if e.Book().Len() != 1 {
		t.Errorf("FOK rejeitada nao deveria tocar o book")
	}
}

func TestFOKFullyFillable(t *testing.T) {
	e := NewEngine(pair)
	e.Process(ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "2", 1))
	e.Process(ord("a2", "s2", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 2))
	r := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.FOK, "100", "3", 3))
	if r.TakerStatus != exchange.StatusFilled || len(r.Trades) != 2 {
		t.Fatalf("FOK com liquidez deveria preencher (2 trades), veio %s trades=%d", r.TakerStatus, len(r.Trades))
	}
}

func TestSelfTradePrevention(t *testing.T) {
	e := NewEngine(pair)
	// alice tem um ask em repouso; alice manda um bid que cruzaria com o proprio ask.
	e.Process(ord("a1", "alice", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 1))
	r := e.Process(ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "100", "1", 2))

	if len(r.Trades) != 0 {
		t.Fatalf("self-trade nao deveria gerar trade, veio %d", len(r.Trades))
	}
	if len(r.CanceledMakers) != 1 || r.CanceledMakers[0].OrderID != "a1" {
		t.Fatalf("o ask da propria conta deveria ser cancelado")
	}
	// O bid (sem contraparte restante) repousa.
	if !r.Rested {
		t.Errorf("o bid deveria repousar apos cancelar o maker proprio")
	}
}

// TestDeterminismReplay garante que processar a MESMA sequencia de comandos em
// dois motores independentes produz exatamente os mesmos trades — base do
// replay via WAL (plano secao 4.2.2).
func TestDeterminismReplay(t *testing.T) {
	build := func() []exchange.Trade {
		e := NewEngine(pair)
		var trades []exchange.Trade
		cmds := []*exchange.Order{
			ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "2", 1),
			ord("a2", "s2", exchange.Sell, exchange.Limit, exchange.GTC, "101", "1", 2),
			ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "100", "1", 3),
			ord("b2", "bob", exchange.Buy, exchange.Limit, exchange.GTC, "101", "3", 4),
		}
		for _, c := range cmds {
			r := e.Process(c)
			trades = append(trades, r.Trades...)
		}
		return trades
	}
	run1 := build()
	run2 := build()
	if len(run1) != len(run2) {
		t.Fatalf("numero de trades difere: %d vs %d", len(run1), len(run2))
	}
	for i := range run1 {
		if run1[i] != run2[i] {
			t.Errorf("trade %d difere entre execucoes:\n %+v\n %+v", i, run1[i], run2[i])
		}
	}
}
