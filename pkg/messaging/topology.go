// Package messaging encapsula a integracao com RabbitMQ:
// envelope de mensagens, publisher com confirms, consumer com ACK manual,
// reconexao automatica e helpers de retry/DLQ.
//
// A topologia (exchanges, filas, bindings) e criada declarativamente pelo
// definitions.json importado no startup do broker - este pacote apenas
// publica e consome.
package messaging

// Constantes da topologia AMQP. Devem espelhar docker/rabbitmq/definitions.json.
const (
	// Exchanges
	ExchangeCommands = "blockchain.commands"
	ExchangeEvents   = "blockchain.events"
	ExchangeDLX      = "blockchain.dlx"
	ExchangeRetry    = "blockchain.retry"

	// Filas
	QueueLeaderCommands = "q.leader.commands"
	QueueGatewayEvents  = "q.gateway.events"
	QueueAuditEvents    = "q.audit.events"
	QueueDLQ            = "q.dlq"
	QueueRetry5s        = "q.retry.5s"

	// Routing keys (Comandos - publicados pelo gateway)
	RKCreditRequested      = "credit.requested"
	RKTransactionRequested = "transaction.requested"

	// Routing keys (Eventos - publicados pelos nos)
	RKCreditAdded         = "credit.added"
	RKTransactionReceived = "transaction.received"
	RKTransactionRejected = "transaction.rejected"
	RKBlockMined          = "block.mined"
	RKLeaderChanged       = "leader.changed"

	// Headers padronizados
	HeaderRetryCount = "x-retry-count"
	HeaderError      = "x-error"

	// Schemas das mensagens
	SchemaCreditRequested      = "blockchain.credit.requested.v1"
	SchemaCreditAdded          = "blockchain.credit.added.v1"
	SchemaTransactionRequested = "blockchain.transaction.requested.v1"
	SchemaTransactionReceived  = "blockchain.transaction.received.v1"
	SchemaTransactionRejected  = "blockchain.transaction.rejected.v1"
	SchemaBlockMined           = "blockchain.block.mined.v1"
	SchemaLeaderChanged        = "blockchain.leader.changed.v1"

	// Limite de tentativas antes de mandar pra DLQ
	MaxRetries = 3
)

// ============================================================================
// Veltra Exchange — topologia de eventos do simulador de exchange.
//
// Reaproveita o mesmo padrao command/event do projeto blockchain (event
// sourcing + CQRS, plano secao 4.4). Comandos sao publicados no exchange de
// comandos e consumidos pelo matching engine (apenas quando lider). O matching
// engine, fonte unica da verdade, emite eventos imutaveis (order.*, trade.*)
// que alimentam as projecoes (ledger, market data, auditoria).
//
// As exchanges/filas/bindings sao declarados em docker/rabbitmq/definitions.json.
// ============================================================================
const (
	// Exchanges da Veltra (topic). Separadas das da blockchain para isolar
	// dominios no mesmo broker.
	ExchangeVeltraCommands = "veltra.commands"
	ExchangeVeltraEvents   = "veltra.events"

	// Filas
	QueueMatchingCommands = "q.matching.commands" // consumida pelo matching engine lider
	QueueLedgerEvents     = "q.ledger.events"     // settlement de dupla entrada
	QueueMarketDataEvents = "q.marketdata.events" // projecao de book/trades/candles
	QueueExchangeAudit    = "q.exchange.audit"    // trilha de auditoria/surveillance

	// Routing keys — Comandos (publicados pelo gateway/OMS)
	RKOrderPlace   = "order.place"
	RKOrderCancel  = "order.cancel"
	RKFaucetCredit = "faucet.credit" // emissao de saldo virtual (admin)

	// Routing keys — Eventos (publicados pelo matching engine / ledger)
	RKOrderAccepted = "order.accepted"
	RKOrderRejected = "order.rejected"
	RKOrderCanceled = "order.canceled"
	RKOrderFilled   = "order.filled"   // fill total ou parcial (execution report)
	RKTradeExecuted = "trade.executed" // casamento entre taker e maker
	RKBookUpdated   = "book.updated"   // delta L2 do order book
	RKLedgerPosted  = "ledger.posted"  // lancamento de dupla entrada confirmado

	// Schemas das mensagens da Veltra
	SchemaOrderPlace    = "veltra.order.place.v1"
	SchemaOrderCancel   = "veltra.order.cancel.v1"
	SchemaFaucetCredit  = "veltra.faucet.credit.v1"
	SchemaOrderAccepted = "veltra.order.accepted.v1"
	SchemaOrderRejected = "veltra.order.rejected.v1"
	SchemaOrderCanceled = "veltra.order.canceled.v1"
	SchemaOrderFilled   = "veltra.order.filled.v1"
	SchemaTradeExecuted = "veltra.trade.executed.v1"
	SchemaBookUpdated   = "veltra.book.updated.v1"
	SchemaLedgerPosted  = "veltra.ledger.posted.v1"
	SchemaMarketUpdate  = "veltra.market.update.v1"

	RKMarketUpdate = "market.update"
)
