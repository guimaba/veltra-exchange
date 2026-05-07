package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publica mensagens com confirms (publisher confirms). Reabre o
// canal automaticamente quando a conexao volta.
type Publisher struct {
	client *Client

	mu sync.Mutex
	ch *amqp.Channel
}

func NewPublisher(client *Client) *Publisher {
	return &Publisher{client: client}
}

// ensureChannel garante um canal aberto com confirms habilitados.
func (p *Publisher) ensureChannel() (*amqp.Channel, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ch != nil && !p.ch.IsClosed() {
		return p.ch, nil
	}

	ch, err := p.client.channel()
	if err != nil {
		return nil, err
	}
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		return nil, fmt.Errorf("falha ao habilitar confirms: %w", err)
	}
	p.ch = ch
	return ch, nil
}

// Publish envia o envelope para o exchange/routing key indicados, com
// publisher confirm e timeout. Retorna erro se o broker nao confirmou em 5s.
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, env Envelope, headers amqp.Table) error {
	ch, err := p.ensureChannel()
	if err != nil {
		return Transient("falha ao obter canal", err)
	}

	body, err := env.Marshal()
	if err != nil {
		return Permanent("falha ao serializar envelope", err)
	}

	if headers == nil {
		headers = amqp.Table{}
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))

	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err = ch.PublishWithContext(publishCtx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		MessageId:    env.TxID,
		Timestamp:    env.Timestamp,
		DeliveryMode: amqp.Persistent,
		Headers:      headers,
		Body:         body,
	})
	if err != nil {
		return Transient("falha ao publicar", err)
	}

	select {
	case conf := <-confirms:
		if !conf.Ack {
			return Transient("broker nao confirmou (NACK)", nil)
		}
		return nil
	case <-publishCtx.Done():
		return Transient("timeout aguardando confirm", publishCtx.Err())
	}
}

// Close fecha o canal do publisher (a conexao subjacente continua viva no Client).
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil && !p.ch.IsClosed() {
		return p.ch.Close()
	}
	return nil
}
