package matching

import (
	"testing"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// bookFingerprint resume o estado do book (precos+qtd+ids) para comparar motores.
func bookFingerprint(e *Engine) []string {
	var fp []string
	for _, o := range e.RestingOrders() {
		fp = append(fp, o.OrderID+":"+string(o.Side)+":"+o.Price.String()+":"+o.Remaining().String())
	}
	return fp
}

func feed(j *Journal, e *Engine, seq *uint64, o *exchange.Order) Result {
	*seq++
	o.Sequence = *seq
	if err := j.Append(Command{Seq: *seq, Kind: CmdPlace, Order: o}); err != nil {
		panic(err)
	}
	return e.Process(o)
}

func TestWALRecovery(t *testing.T) {
	dir := t.TempDir()

	// Sessao 1: abre, processa comandos, fecha SEM snapshot (simula crash).
	j, e, last, err := OpenJournal(dir, pair)
	if err != nil {
		t.Fatal(err)
	}
	if last != 0 {
		t.Fatalf("journal novo deveria ter lastSeq 0, veio %d", last)
	}
	var seq uint64
	feed(j, e, &seq, ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "2", 0))
	feed(j, e, &seq, ord("a2", "s2", exchange.Sell, exchange.Limit, exchange.GTC, "101", "1", 0))
	feed(j, e, &seq, ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "100", "1", 0))
	want := bookFingerprint(e)
	j.Close()

	// Sessao 2: reabre -> deve reconstruir o mesmo estado via replay do WAL.
	j2, e2, last2, err := OpenJournal(dir, pair)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	if last2 != seq {
		t.Errorf("lastSeq recuperado = %d, quero %d", last2, seq)
	}
	got := bookFingerprint(e2)
	if len(got) != len(want) {
		t.Fatalf("book recuperado difere em tamanho: %v vs %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("nivel %d difere: %q vs %q", i, got[i], want[i])
		}
	}
}

func TestSnapshotTruncatesWAL(t *testing.T) {
	dir := t.TempDir()
	j, e, _, err := OpenJournal(dir, pair)
	if err != nil {
		t.Fatal(err)
	}
	var seq uint64
	feed(j, e, &seq, ord("a1", "s1", exchange.Sell, exchange.Limit, exchange.GTC, "100", "1", 0))
	feed(j, e, &seq, ord("a2", "s2", exchange.Sell, exchange.Limit, exchange.GTC, "101", "1", 0))

	if err := j.Snapshot(e, seq); err != nil {
		t.Fatal(err)
	}
	// Apos snapshot o WAL deve estar vazio.
	cmds, err := readWAL(j.walPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 0 {
		t.Errorf("WAL deveria estar truncado, tem %d comandos", len(cmds))
	}

	// Mais um comando apos o snapshot.
	feed(j, e, &seq, ord("b1", "alice", exchange.Buy, exchange.Limit, exchange.GTC, "100", "1", 0))
	want := bookFingerprint(e)
	j.Close()

	// Reabre: snapshot + replay do unico comando pos-snapshot.
	j2, e2, last2, err := OpenJournal(dir, pair)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	if last2 != seq {
		t.Errorf("lastSeq = %d, quero %d", last2, seq)
	}
	got := bookFingerprint(e2)
	if len(got) != len(want) {
		t.Fatalf("estado pos-recuperacao difere: %v vs %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("nivel %d: %q vs %q", i, got[i], want[i])
		}
	}
	// b1 (100) casou com a1 (100)? Nao: a1 e ask 100, b1 bid 100 -> casa!
	// Logo apos recuperacao, a1 sumiu e sobra a2(101). Verifica consistencia.
	if _, ok := e2.Book().BestAsk(); !ok {
		t.Error("deveria restar um ask (a2) apos o casamento de a1/b1")
	}
	_ = money.Zero
}
