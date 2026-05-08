package messaging

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publica mensagens com publisher confirms. Reabre o canal
// automaticamente quando a conexao volta.
//
// Importante: amqp091.NotifyPublish faz send BLOQUEANTE para todos os
// listeners registrados. Por isso registramos UM unico listener por canal e
// SERIALIZAMOS publishes com mutex - assim cada Publish() sempre le seu
// proprio confirm, sem races nem listeners orfaos.
type Publisher struct {
	client *Client

	mu       sync.Mutex // serializa publishes; protege ch e confirms
	ch       *amqp.Channel
	confirms chan amqp.Confirmation
}

func NewPublisher(client *Client) *Publisher {
	return &Publisher{client: client}
}

// ensureChannelLocked garante um canal aberto com confirms habilitados.
// Caller deve segurar p.mu.
func (p *Publisher) ensureChannelLocked() (*amqp.Channel, chan amqp.Confirmation, error) {
	if p.ch != nil && !p.ch.IsClosed() {
		return p.ch, p.confirms, nil
	}

	ch, err := p.client.channel()
	if err != nil {
		return nil, nil, err
	}
	if err := ch.Confirm(false); err != nil {
		ch.Close()
		return nil, nil, fmt.Errorf("falha ao habilitar confirms: %w", err)
	}
	// Buffer pequeno e suficiente: como serializamos publishes, so um confirm
	// fica pendente por vez.
	p.ch = ch
	p.confirms = ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	return p.ch, p.confirms, nil
}

// Publish envia o envelope para o exchange/routing key indicados, com
// publisher confirm e timeout.
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, env Envelope, headers amqp.Table) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ch, confirms, err := p.ensureChannelLocked()
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
		// Canal pode ter morrido; force recriacao no proximo publish.
		p.invalidateChannel()
		return Transient("falha ao publicar", err)
	}

	select {
	case conf, ok := <-confirms:
		if !ok {
			// Canal foi fechado; recria no proximo publish.
			p.invalidateChannel()
			return Transient("canal de confirms fechado", nil)
		}
		if !conf.Ack {
			return Transient("broker nao confirmou (NACK)", nil)
		}
		return nil
	case <-publishCtx.Done():
		// Timeout: o canal pode estar em estado ruim. Forca reabrir.
		p.invalidateChannel()
		return Transient("timeout aguardando confirm", publishCtx.Err())
	}
}

// invalidateChannel marca o canal pra ser recriado no proximo publish.
// Deve ser chamado com mutex segurado.
func (p *Publisher) invalidateChannel() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	p.ch = nil
	p.confirms = nil
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
