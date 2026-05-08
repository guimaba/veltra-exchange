package messaging

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Client gerencia a conexao com o RabbitMQ. Reconecta automaticamente em caso
// de queda, com backoff exponencial limitado a 30s.
//
// Uma instancia por processo - conexoes AMQP sao caras, canais sao baratos.
// Use NewPublisher / Consume para obter handles especificos.
type Client struct {
	url string

	mu    sync.RWMutex
	conn  *amqp.Connection
	ready chan struct{} // fechado quando ha conexao viva; recriado em reconexoes
	done  chan struct{} // fechado em Close()
	once  sync.Once
}

// NewClient cria o client e dispara a goroutine de manutencao da conexao.
// Bloqueia ate a primeira conexao ser estabelecida ou ate o ctx ser cancelado.
func NewClient(ctx context.Context, url string) (*Client, error) {
	c := &Client{
		url:   url,
		ready: make(chan struct{}),
		done:  make(chan struct{}),
	}
	go c.maintain()

	// Aguarda primeira conexao
	select {
	case <-c.ready:
		return c, nil
	case <-ctx.Done():
		c.Close()
		return nil, ctx.Err()
	}
}

// maintain roda em goroutine; reconecta com backoff ate Close().
func (c *Client) maintain() {
	backoff := 1 * time.Second
	for {
		select {
		case <-c.done:
			return
		default:
		}

		conn, err := amqp.DialConfig(c.url, amqp.Config{
			Heartbeat: 30 * time.Second,
			Locale:    "en_US",
		})
		if err != nil {
			log.Printf("[Messaging] Falha ao conectar: %v. Tentando em %s...", err, backoff)
			select {
			case <-time.After(backoff):
			case <-c.done:
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}

		log.Printf("[Messaging] Conectado ao RabbitMQ")
		backoff = 1 * time.Second

		c.mu.Lock()
		c.conn = conn
		// Sinaliza que esta pronto (fecha o canal)
		select {
		case <-c.ready:
			c.ready = make(chan struct{})
		default:
		}
		close(c.ready)
		c.mu.Unlock()

		// Bloqueia ate a conexao cair
		closeErr := <-conn.NotifyClose(make(chan *amqp.Error, 1))
		log.Printf("[Messaging] Conexao perdida: %v", closeErr)

		c.mu.Lock()
		c.conn = nil
		c.ready = make(chan struct{}) // novo canal pra proxima vez
		c.mu.Unlock()
	}
}

// channel obtem um canal AMQP novo. Bloqueia se a conexao estiver offline.
func (c *Client) channel() (*amqp.Channel, error) {
	c.mu.RLock()
	ready := c.ready
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		// Aguarda reconexao
		select {
		case <-ready:
		case <-c.done:
			return nil, fmt.Errorf("client fechado")
		case <-time.After(60 * time.Second):
			return nil, fmt.Errorf("timeout aguardando conexao com RabbitMQ")
		}
		c.mu.RLock()
		conn = c.conn
		c.mu.RUnlock()
		if conn == nil {
			return nil, fmt.Errorf("conexao indisponivel")
		}
	}

	return conn.Channel()
}

// Close encerra o client e a conexao subjacente.
func (c *Client) Close() error {
	c.once.Do(func() {
		close(c.done)
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	})
	return nil
}
