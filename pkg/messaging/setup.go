package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DeclareTopology cria, de forma idempotente, todas as exchanges, filas e
// bindings da Veltra (e da blockchain legada) que normalmente são carregadas
// pelo definitions.json no Docker Compose.
//
// O Amazon MQ (RabbitMQ gerenciado) NÃO importa o definitions.json, então cada
// serviço declara a topologia no startup. Declarar algo já existente é no-op
// (desde que os argumentos batam), então qualquer serviço pode chamar.
func (c *Client) DeclareTopology() error {
	ch, err := c.channel()
	if err != nil {
		return fmt.Errorf("DeclareTopology channel: %w", err)
	}
	defer ch.Close()

	// ---- Exchanges ----
	exchanges := []struct {
		name, kind     string
		internal       bool
	}{
		{ExchangeCommands, "direct", false}, // blockchain.commands
		{ExchangeEvents, "topic", false},    // blockchain.events
		{ExchangeDLX, "fanout", false},      // blockchain.dlx
		{ExchangeRetry, "direct", true},     // blockchain.retry (internal)
		{ExchangeVeltraCommands, "topic", false},
		{ExchangeVeltraEvents, "topic", false},
	}
	for _, e := range exchanges {
		if err := ch.ExchangeDeclare(e.name, e.kind, true, false, e.internal, false, nil); err != nil {
			return fmt.Errorf("declare exchange %s: %w", e.name, err)
		}
	}

	// ---- Filas (com argumentos do definitions.json) ----
	queues := []struct {
		name string
		args amqp.Table
	}{
		{QueueLeaderCommands, amqp.Table{"x-message-ttl": int32(60000), "x-dead-letter-exchange": ExchangeDLX, "x-max-length": int32(10000), "x-overflow": "reject-publish"}},
		{QueueGatewayEvents, amqp.Table{"x-max-length": int32(5000), "x-overflow": "drop-head"}},
		{QueueAuditEvents, amqp.Table{"x-message-ttl": int32(86400000)}},
		{QueueDLQ, amqp.Table{"x-message-ttl": int32(604800000)}},
		{QueueRetry5s, amqp.Table{"x-message-ttl": int32(5000), "x-dead-letter-exchange": ExchangeCommands}},
		{QueueMatchingCommands, amqp.Table{"x-message-ttl": int32(60000), "x-dead-letter-exchange": ExchangeDLX, "x-max-length": int32(50000), "x-overflow": "reject-publish"}},
		{QueueLedgerEvents, amqp.Table{"x-dead-letter-exchange": ExchangeDLX, "x-max-length": int32(100000), "x-overflow": "reject-publish"}},
		{QueueMarketDataEvents, amqp.Table{"x-max-length": int32(50000), "x-overflow": "drop-head"}},
		{QueueExchangeAudit, amqp.Table{"x-message-ttl": int32(86400000)}},
	}
	for _, q := range queues {
		if _, err := ch.QueueDeclare(q.name, true, false, false, false, q.args); err != nil {
			return fmt.Errorf("declare queue %s: %w", q.name, err)
		}
	}

	// ---- Bindings ----
	binds := []struct{ queue, key, exchange string }{
		{QueueLeaderCommands, RKCreditRequested, ExchangeCommands},
		{QueueLeaderCommands, RKTransactionRequested, ExchangeCommands},
		{QueueGatewayEvents, "#", ExchangeEvents},
		{QueueAuditEvents, "#", ExchangeEvents},
		{QueueDLQ, "", ExchangeDLX},
		{QueueRetry5s, RKCreditRequested, ExchangeRetry},
		{QueueRetry5s, RKTransactionRequested, ExchangeRetry},
		{QueueMatchingCommands, RKOrderPlace, ExchangeVeltraCommands},
		{QueueMatchingCommands, RKOrderCancel, ExchangeVeltraCommands},
		{QueueLedgerEvents, RKTradeExecuted, ExchangeVeltraEvents},
		{QueueLedgerEvents, RKFaucetCredit, ExchangeVeltraEvents},
		{QueueMarketDataEvents, "#", ExchangeVeltraEvents},
		{QueueExchangeAudit, "#", ExchangeVeltraEvents},
	}
	for _, b := range binds {
		if err := ch.QueueBind(b.queue, b.key, b.exchange, false, nil); err != nil {
			return fmt.Errorf("bind %s->%s: %w", b.exchange, b.queue, err)
		}
	}

	return nil
}
