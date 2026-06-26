// Comando: servico de auditoria.
//
// Consome todos os eventos de blockchain.events (via q.audit.events) e
// persiste cada um na tabela audit_events do MariaDB. E idempotente via a
// tabela processed_messages (PK composta tx_id+consumer).
//
// Configuracao via env vars:
//
//	AMQP_URL (obrigatorio)
//	DB_DSN   (obrigatorio) - "user:pass@tcp(host:port)/blockchain?parseTime=true"
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
)

func main() {
	amqpURL := os.Getenv("AMQP_URL")
	dbDSN := os.Getenv("DB_DSN")
	if amqpURL == "" || dbDSN == "" {
		log.Fatal("AMQP_URL e DB_DSN sao obrigatorios")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Conecta no MariaDB (com retry interno enquanto o container sobe)
	storeCtx, storeCancel := context.WithTimeout(ctx, 60*time.Second)
	store, err := NewStore(storeCtx, dbDSN)
	storeCancel()
	if err != nil {
		log.Fatalf("[Audit] Falha ao conectar ao MariaDB: %v", err)
	}
	defer store.Close()
	log.Printf("[Audit] Conectado ao MariaDB")

	// Conecta no Rabbit
	bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
	client, err := messaging.NewClient(bootCtx, amqpURL)
	bootCancel()
	if err != nil {
		log.Fatalf("[Audit] Falha ao conectar ao RabbitMQ: %v", err)
	}
	defer client.Close()

	publisher := messaging.NewPublisher(client)
	defer publisher.Close()

	consumer := messaging.NewConsumer(
		client,
		publisher,
		messaging.ConsumeOptions{
			Queue:    messaging.QueueAuditEvents,
			Consumer: "audit",
			Prefetch: 50, // auditoria suporta paralelismo - eventos sao independentes
		},
		handler(store),
	)
	consumer.Start(ctx)

	// Trilha de auditoria da Veltra Exchange (plano §5.2 — "log de eventos
	// permite reconstruir e auditar qualquer estado"). Consome q.exchange.audit
	// (todos os eventos da exchange) e persiste os eventos de negócio.
	veltraAudit := messaging.NewConsumer(
		client,
		publisher,
		messaging.ConsumeOptions{
			Queue:    messaging.QueueExchangeAudit,
			Consumer: "audit-veltra",
			Prefetch: 50,
		},
		veltraHandler(store),
	)
	veltraAudit.Start(ctx)

	log.Printf("[Audit] Pronto. Eventos em %s e %s.", messaging.QueueAuditEvents, messaging.QueueExchangeAudit)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("[Audit] Encerrando...")
	consumer.Stop()
	veltraAudit.Stop()
}

// veltraHandler persiste os eventos de NEGÓCIO da Veltra (order.*, trade.executed,
// faucet.credit, ledger.posted). Ignora projeções de alto volume (book.updated,
// market.update) que não têm valor de auditoria e inflariam a tabela.
func veltraHandler(store *Store) messaging.Handler {
	skip := map[string]bool{
		messaging.RKBookUpdated:  true,
		messaging.RKMarketUpdate: true,
	}
	return func(ctx context.Context, env messaging.Envelope, d amqp.Delivery) error {
		if skip[d.RoutingKey] {
			return nil // ACK sem persistir
		}
		if err := store.SaveEvent(ctx, env.Schema, env.TxID, env.Payload); err != nil {
			if isTransientDB(err) {
				return messaging.Transient("falha ao persistir audit veltra", err)
			}
			return messaging.Permanent("erro persistente ao gravar audit veltra", err)
		}
		log.Printf("[Audit/Veltra] Persistido: schema=%s rk=%s tx=%s", env.Schema, d.RoutingKey, env.TxID)
		return nil
	}
}

// handler processa cada envelope persistindo em audit_events. Erros de DB
// sao classificados como Transient (retry). Payloads invalidos vao pra DLQ.
func handler(store *Store) messaging.Handler {
	return func(ctx context.Context, env messaging.Envelope, d amqp.Delivery) error {
		err := store.SaveEvent(ctx, env.Schema, env.TxID, env.Payload)
		if err == nil {
			log.Printf("[Audit] Persistido: schema=%s tx=%s rk=%s", env.Schema, env.TxID, d.RoutingKey)
			return nil
		}
		// Erros de DB sao transitorios na maioria dos casos (lock, deadlock,
		// indisponibilidade). Deixa o retry pipeline cuidar.
		if isTransientDB(err) {
			return messaging.Transient("falha ao persistir evento", err)
		}
		return messaging.Permanent("erro persistente ao gravar audit", err)
	}
}

// isTransientDB identifica erros de DB que se beneficiam de retry.
// Estrategia simples: tudo que nao e nil e tratado como transitorio.
// Refinar depois caso vejamos padroes especificos.
func isTransientDB(err error) bool {
	if err == nil {
		return false
	}
	// Erros de constraint violation NUNCA se beneficiam de retry, mas a
	// idempotencia ja trata isso via INSERT IGNORE em markProcessed.
	// Os demais cenarios (timeout, deadlock, conexao perdida) sao transitorios.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return true
}
