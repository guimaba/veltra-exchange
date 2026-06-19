package main

// service.go: o nucleo do matching engine como servico. Mantem um motor +
// journal (WAL/snapshot) por par de negociacao. Apenas o lider opera; o
// processamento e single-threaded por construcao (consumer com prefetch=1
// entrega uma mensagem por vez), preservando o determinismo do plano (4.2.2).

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/matching"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// pairEngine agrupa o motor de um par com seu journal e a sequencia aplicada.
type pairEngine struct {
	pair          exchange.Pair
	engine        *matching.Engine
	journal       *matching.Journal
	lastSeq       uint64
	sinceSnapshot int
}

// MatchingService coordena os motores de todos os pares configurados.
type MatchingService struct {
	nodeID        int
	walDir        string
	snapshotEvery int
	publisher     *messaging.Publisher

	mu      sync.Mutex
	engines map[string]*pairEngine // chave = pair.String()
	pairs   []exchange.Pair
	active  bool // true enquanto lider (journals abertos)
}

func NewMatchingService(nodeID int, walDir string, snapshotEvery int, pairs []exchange.Pair, pub *messaging.Publisher) *MatchingService {
	return &MatchingService{
		nodeID:        nodeID,
		walDir:        walDir,
		snapshotEvery: snapshotEvery,
		publisher:     pub,
		engines:       make(map[string]*pairEngine),
		pairs:         pairs,
	}
}

// Activate (re)carrega os motores do WAL compartilhado ao virar lider.
func (s *MatchingService) Activate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return nil
	}
	for _, p := range s.pairs {
		j, eng, lastSeq, err := matching.OpenJournal(s.walDir, p)
		if err != nil {
			return fmt.Errorf("abrir journal de %s: %w", p, err)
		}
		s.engines[p.String()] = &pairEngine{pair: p, engine: eng, journal: j, lastSeq: lastSeq}
		log.Printf("[Matching %d] Par %s recuperado (lastSeq=%d, ordens=%d)", s.nodeID, p, lastSeq, eng.Book().Len())
	}
	s.active = true
	return nil
}

// Deactivate fecha os journals ao perder lideranca (faz snapshot final).
func (s *MatchingService) Deactivate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return
	}
	for _, pe := range s.engines {
		_ = pe.journal.Snapshot(pe.engine, pe.lastSeq)
		_ = pe.journal.Close()
	}
	s.engines = make(map[string]*pairEngine)
	s.active = false
	log.Printf("[Matching %d] Motores desativados (standby).", s.nodeID)
}

// Handler retorna o messaging.Handler que processa order.place / order.cancel.
func (s *MatchingService) Handler() messaging.Handler {
	return func(ctx context.Context, env messaging.Envelope, d amqp.Delivery) error {
		switch d.RoutingKey {
		case messaging.RKOrderPlace:
			return s.handlePlace(ctx, env)
		case messaging.RKOrderCancel:
			return s.handleCancel(ctx, env)
		default:
			return messaging.Permanent("routing key desconhecida: "+d.RoutingKey, nil)
		}
	}
}

func (s *MatchingService) handlePlace(ctx context.Context, env messaging.Envelope) error {
	var p messaging.OrderPlacePayload
	if err := env.Unmarshal(&p); err != nil {
		return messaging.Permanent("payload order.place invalido", err)
	}
	pair, err := exchange.ParsePair(p.Pair)
	if err != nil {
		return messaging.Business("par invalido: " + p.Pair)
	}

	s.mu.Lock()
	pe, ok := s.engines[pair.String()]
	if !ok {
		s.mu.Unlock()
		return messaging.Business("par nao suportado: " + pair.String())
	}

	// Sequencia + timestamp atribuidos na ingestao e GRAVADOS no WAL antes de
	// processar. O replay le esses mesmos valores -> determinismo.
	pe.lastSeq++
	order := &exchange.Order{
		OrderID:       uuid.NewString(),
		ClientOrderID: p.ClientOrderID,
		Account:       p.Account,
		Pair:          pair,
		Side:          exchange.Side(p.Side),
		Type:          exchange.OrderType(p.Type),
		TimeInForce:   exchange.TimeInForce(p.TimeInForce),
		Price:         money.Amount(p.Price),
		Quantity:      money.Amount(p.Quantity),
		Status:        exchange.StatusNew,
		Sequence:      pe.lastSeq,
		TimestampMs:   time.Now().UnixMilli(),
	}

	if err := pe.journal.Append(matching.Command{Seq: pe.lastSeq, Kind: matching.CmdPlace, Order: order}); err != nil {
		pe.lastSeq--
		s.mu.Unlock()
		return messaging.Transient("falha ao gravar no WAL", err)
	}

	res := pe.engine.Process(order)
	pe.maybeSnapshot(s.snapshotEvery)
	book := captureBook(pe)
	s.mu.Unlock()

	s.publishPlaceResult(ctx, order, res)
	s.publishBook(ctx, book)
	return nil
}

func (s *MatchingService) handleCancel(ctx context.Context, env messaging.Envelope) error {
	var p messaging.OrderCancelPayload
	if err := env.Unmarshal(&p); err != nil {
		return messaging.Permanent("payload order.cancel invalido", err)
	}
	pair, err := exchange.ParsePair(p.Pair)
	if err != nil {
		return messaging.Business("par invalido: " + p.Pair)
	}

	s.mu.Lock()
	pe, ok := s.engines[pair.String()]
	if !ok {
		s.mu.Unlock()
		return messaging.Business("par nao suportado: " + pair.String())
	}

	pe.lastSeq++
	if err := pe.journal.Append(matching.Command{Seq: pe.lastSeq, Kind: matching.CmdCancel, OrderID: p.OrderID}); err != nil {
		pe.lastSeq--
		s.mu.Unlock()
		return messaging.Transient("falha ao gravar no WAL", err)
	}

	canceled, found := pe.engine.Cancel(p.OrderID)
	pe.maybeSnapshot(s.snapshotEvery)
	book := captureBook(pe)
	s.mu.Unlock()
	defer s.publishBook(ctx, book)

	if !found {
		s.emit(ctx, messaging.RKOrderRejected, messaging.SchemaOrderRejected, env.TxID, messaging.OrderRejectedPayload{
			ClientOrderID: p.ClientOrderID,
			Account:       p.Account,
			Pair:          pair.String(),
			Reason:        "ORDER_NOT_FOUND",
		})
		return nil
	}
	s.emit(ctx, messaging.RKOrderCanceled, messaging.SchemaOrderCanceled, canceled.OrderID, messaging.OrderCanceledPayload{
		OrderID: canceled.OrderID,
		Account: canceled.Account,
		Pair:    pair.String(),
	})
	return nil
}

// maybeSnapshot dispara um snapshot a cada snapshotEvery comandos aplicados.
func (pe *pairEngine) maybeSnapshot(every int) {
	pe.sinceSnapshot++
	if every > 0 && pe.sinceSnapshot >= every {
		if err := pe.journal.Snapshot(pe.engine, pe.lastSeq); err != nil {
			log.Printf("[Matching] Falha ao snapshotar %s: %v", pe.pair, err)
			return
		}
		pe.sinceSnapshot = 0
	}
}

// publishPlaceResult emite os eventos resultantes de um order.place.
func (s *MatchingService) publishPlaceResult(ctx context.Context, o *exchange.Order, res matching.Result) {
	if res.TakerStatus == exchange.StatusRejected {
		s.emit(ctx, messaging.RKOrderRejected, messaging.SchemaOrderRejected, o.OrderID, messaging.OrderRejectedPayload{
			ClientOrderID: o.ClientOrderID,
			Account:       o.Account,
			Pair:          o.Pair.String(),
			Reason:        res.RejectReason,
		})
		return
	}

	s.emit(ctx, messaging.RKOrderAccepted, messaging.SchemaOrderAccepted, o.OrderID, messaging.OrderAcceptedPayload{
		OrderID:       o.OrderID,
		ClientOrderID: o.ClientOrderID,
		Account:       o.Account,
		Pair:          o.Pair.String(),
		Side:          string(o.Side),
		Type:          string(o.Type),
		Price:         int64(o.Price),
		Quantity:      int64(o.Quantity),
		Sequence:      o.Sequence,
	})

	// Makers cancelados por self-trade prevention.
	for _, m := range res.CanceledMakers {
		s.emit(ctx, messaging.RKOrderCanceled, messaging.SchemaOrderCanceled, m.OrderID, messaging.OrderCanceledPayload{
			OrderID: m.OrderID,
			Account: m.Account,
			Pair:    o.Pair.String(),
		})
	}

	// Trades (fonte da verdade para ledger e market data).
	for _, t := range res.Trades {
		s.emit(ctx, messaging.RKTradeExecuted, messaging.SchemaTradeExecuted, t.TradeID, messaging.TradeExecutedPayload{
			TradeID:      t.TradeID,
			Pair:         t.Pair.String(),
			Price:        int64(t.Price),
			Quantity:     int64(t.Quantity),
			TakerOrderID: t.TakerOrderID,
			MakerOrderID: t.MakerOrderID,
			TakerAccount: t.TakerAccount,
			MakerAccount: t.MakerAccount,
			TakerSide:    string(t.TakerSide),
			Sequence:     t.Sequence,
			TimestampMs:  t.TimestampMs,
		})
	}

	// Execution reports (taker e makers).
	for _, f := range res.Fills {
		s.emit(ctx, messaging.RKOrderFilled, messaging.SchemaOrderFilled, f.OrderID, messaging.OrderFilledPayload{
			OrderID:          f.OrderID,
			ClientOrderID:    f.ClientOrderID,
			Account:          f.Account,
			Pair:             o.Pair.String(),
			Side:             string(f.Side),
			Price:            int64(f.Price),
			FillQuantity:     int64(f.FillQty),
			CumulativeFilled: int64(f.CumulativeFilled),
			RemainingQty:     int64(f.Remaining),
			Status:           string(f.Status),
		})
	}

	// Remanescente de market/IOC nao repousa: avisa cancelamento do saldo.
	if !res.Rested && res.TakerStatus != exchange.StatusFilled && o.Remaining().IsPositive() {
		s.emit(ctx, messaging.RKOrderCanceled, messaging.SchemaOrderCanceled, o.OrderID, messaging.OrderCanceledPayload{
			OrderID: o.OrderID,
			Account: o.Account,
			Pair:    o.Pair.String(),
		})
	}
}

// bookDepth limita os niveis L2 publicados em book.updated.
const bookDepth = 20

// captureBook tira uma foto L2 do book. Deve ser chamado com s.mu segurado;
// o publish acontece fora do lock.
func captureBook(pe *pairEngine) messaging.BookUpdatedPayload {
	bids, asks := pe.engine.Book().L2(bookDepth)
	out := messaging.BookUpdatedPayload{
		Pair:     pe.pair.String(),
		Bids:     make([]messaging.BookLevel, 0, len(bids)),
		Asks:     make([]messaging.BookLevel, 0, len(asks)),
		Sequence: pe.lastSeq,
	}
	for _, lv := range bids {
		out.Bids = append(out.Bids, messaging.BookLevel{Price: int64(lv.Price), Quantity: int64(lv.Quantity)})
	}
	for _, lv := range asks {
		out.Asks = append(out.Asks, messaging.BookLevel{Price: int64(lv.Price), Quantity: int64(lv.Quantity)})
	}
	return out
}

// publishBook emite o snapshot L2 (book.updated) para as projecoes de market data.
func (s *MatchingService) publishBook(ctx context.Context, book messaging.BookUpdatedPayload) {
	s.emit(ctx, messaging.RKBookUpdated, messaging.SchemaBookUpdated, "", book)
}

// emit publica um evento no exchange de eventos da Veltra.
func (s *MatchingService) emit(ctx context.Context, rk, schema, txID string, payload any) {
	env, err := messaging.NewEnvelope(schema, txID, payload)
	if err != nil {
		log.Printf("[Matching %d] Falha ao montar envelope %s: %v", s.nodeID, schema, err)
		return
	}
	if err := s.publisher.Publish(ctx, messaging.ExchangeVeltraEvents, rk, env, nil); err != nil {
		log.Printf("[Matching %d] Falha ao publicar %s: %v", s.nodeID, rk, err)
	}
}
