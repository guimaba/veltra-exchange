package messaging

import (
	"context"
	"errors"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler processa uma mensagem AMQP. Retorna um Error tipado para o framework
// decidir entre ACK / retry / DLQ. Erro nil = ACK direto.
type Handler func(ctx context.Context, env Envelope, delivery amqp.Delivery) error

// ConsumeOptions parametriza o Consume.
type ConsumeOptions struct {
	Queue     string // fila a consumir
	Consumer  string // nome do consumer (aparece no painel)
	Prefetch  int    // QoS prefetch count (default 1)
	AutoStart bool   // se true, inicia imediatamente
}

// Consumer roda um Handler contra mensagens de uma fila. Reanexa automaticamente
// quando a conexao volta.
type Consumer struct {
	client    *Client
	publisher *Publisher
	opts      ConsumeOptions
	handler   Handler

	cancel context.CancelFunc
}

// NewConsumer cria um consumer; chame Start() para comecar.
// O publisher e usado para retry / DLQ / publicacao de eventos correlatos.
func NewConsumer(client *Client, publisher *Publisher, opts ConsumeOptions, h Handler) *Consumer {
	if opts.Prefetch <= 0 {
		opts.Prefetch = 1
	}
	if opts.Consumer == "" {
		opts.Consumer = opts.Queue + "-consumer"
	}
	return &Consumer{client: client, publisher: publisher, opts: opts, handler: h}
}

// Start inicia o loop de consumo em uma goroutine. Cancele com Stop() ou
// cancelando o ctx pai.
func (c *Consumer) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel

	go func() {
		for {
			if err := ctx.Err(); err != nil {
				return
			}
			if err := c.runOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("[Messaging/%s] Loop interrompido: %v. Reanexando em 2s...", c.opts.Queue, err)
				select {
				case <-time.After(2 * time.Second):
				case <-ctx.Done():
					return
				}
			}
		}
	}()
}

// Stop cancela o loop de consumo.
func (c *Consumer) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// runOnce abre um canal, registra consumer, processa ate ele fechar.
func (c *Consumer) runOnce(ctx context.Context) error {
	ch, err := c.client.channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if err := ch.Qos(c.opts.Prefetch, 0, false); err != nil {
		return err
	}

	deliveries, err := ch.Consume(c.opts.Queue, c.opts.Consumer, false, false, false, false, nil)
	if err != nil {
		return err
	}

	log.Printf("[Messaging/%s] Consumindo (prefetch=%d)", c.opts.Queue, c.opts.Prefetch)

	closed := ch.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-closed:
			return err
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel fechado")
			}
			c.dispatch(ctx, d)
		}
	}
}

// dispatch parseia a mensagem e chama o handler, decidindo ACK / retry / DLQ.
func (c *Consumer) dispatch(ctx context.Context, d amqp.Delivery) {
	env, err := ParseEnvelope(d.Body)
	if err != nil {
		log.Printf("[Messaging/%s] Mensagem mal-formada -> DLQ. err=%v", c.opts.Queue, err)
		c.toDLQ(ctx, d, "envelope_parse_error: "+err.Error())
		_ = d.Ack(false)
		return
	}

	herr := c.handler(ctx, env, d)
	if herr == nil {
		_ = d.Ack(false)
		return
	}

	var typed *Error
	if !errors.As(herr, &typed) {
		// Erro nao classificado: tratamos como transitorio.
		typed = Transient("erro nao classificado", herr)
	}

	switch typed.Kind {
	case KindBusiness:
		log.Printf("[Messaging/%s] Erro de negocio (tx=%s): %s. ACK.", c.opts.Queue, env.TxID, typed.Reason)
		_ = d.Ack(false)
	case KindPermanent:
		log.Printf("[Messaging/%s] Erro permanente (tx=%s): %v -> DLQ.", c.opts.Queue, env.TxID, typed)
		c.toDLQ(ctx, d, typed.Error())
		_ = d.Ack(false)
	case KindTransient:
		retries := readRetryCount(d.Headers)
		if retries >= MaxRetries {
			log.Printf("[Messaging/%s] Excedeu retries (tx=%s) -> DLQ.", c.opts.Queue, env.TxID)
			c.toDLQ(ctx, d, "max_retries_exceeded: "+typed.Error())
		} else {
			log.Printf("[Messaging/%s] Erro transitorio (tx=%s, tentativa %d/%d): %v. Retry.", c.opts.Queue, env.TxID, retries+1, MaxRetries, typed)
			c.toRetry(ctx, d, retries+1, typed.Error())
		}
		_ = d.Ack(false)
	}
}

// toRetry republica em blockchain.retry preservando routing key e incrementando contador.
func (c *Consumer) toRetry(ctx context.Context, d amqp.Delivery, count int, reason string) {
	headers := copyHeaders(d.Headers)
	headers[HeaderRetryCount] = int32(count)
	headers[HeaderError] = reason

	env, _ := ParseEnvelope(d.Body)
	if err := c.publisher.Publish(ctx, ExchangeRetry, d.RoutingKey, env, headers); err != nil {
		log.Printf("[Messaging/%s] Falha ao publicar em retry: %v. Mandando direto pra DLQ.", c.opts.Queue, err)
		c.toDLQ(ctx, d, "retry_publish_failed: "+reason)
	}
}

// toDLQ publica diretamente em blockchain.dlx (fanout -> q.dlq).
func (c *Consumer) toDLQ(ctx context.Context, d amqp.Delivery, reason string) {
	headers := copyHeaders(d.Headers)
	headers[HeaderError] = reason
	headers["x-original-routing-key"] = d.RoutingKey
	headers["x-original-exchange"] = d.Exchange

	env, _ := ParseEnvelope(d.Body)
	if err := c.publisher.Publish(ctx, ExchangeDLX, "", env, headers); err != nil {
		log.Printf("[Messaging/%s] FALHA CRITICA: nao consegui publicar na DLQ: %v", c.opts.Queue, err)
	}
}

func readRetryCount(h amqp.Table) int {
	if h == nil {
		return 0
	}
	v, ok := h[HeaderRetryCount]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int32:
		return int(n)
	case int64:
		return int(n)
	}
	return 0
}

func copyHeaders(h amqp.Table) amqp.Table {
	out := amqp.Table{}
	for k, v := range h {
		out[k] = v
	}
	return out
}
