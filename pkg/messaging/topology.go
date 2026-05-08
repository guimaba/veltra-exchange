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
