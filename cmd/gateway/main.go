// Comando: Gateway HTTP/WebSocket.
//
// Funcoes:
//   - Expoe API REST (/api/*) que recebe comandos do Flutter Web e publica
//     no exchange blockchain.commands.
//   - Mantem snapshot em memoria do estado da rede (saldos, blocos, lider)
//     consumindo q.gateway.events.
//   - Faz broadcast WebSocket dos eventos para todos os clientes conectados.
//   - Serve os arquivos estaticos do Flutter Web em /.
//
// Configuracao via env vars:
//
//	GATEWAY_PORT (default 8080)
//	AMQP_URL     (obrigatorio)
//	STATIC_DIR   (default ./static; se nao existir, /  retorna 404)
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
)

func main() {
	port := envOr("GATEWAY_PORT", "8080")
	amqpURL := os.Getenv("AMQP_URL")
	staticDir := envOr("STATIC_DIR", "./static")

	if amqpURL == "" {
		log.Fatal("AMQP_URL e obrigatoria")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Conecta no Rabbit
	bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
	client, err := messaging.NewClient(bootCtx, amqpURL)
	bootCancel()
	if err != nil {
		log.Fatalf("[Gateway] Falha ao conectar ao RabbitMQ: %v", err)
	}
	defer client.Close()

	publisher := messaging.NewPublisher(client)
	defer publisher.Close()

	// Estado + Hub
	state := NewState()
	hub := NewHub()
	go hub.Run()

	// Consumer dos eventos -> atualiza state + hub
	consumer := messaging.NewConsumer(
		client,
		publisher,
		messaging.ConsumeOptions{
			Queue:    messaging.QueueGatewayEvents,
			Consumer: "gateway",
			Prefetch: 10,
		},
		func(ctx context.Context, env messaging.Envelope, d amqp.Delivery) error {
			state.ApplyEvent(d.RoutingKey, env)
			hub.PushEvent(d.RoutingKey, json.RawMessage(env.Payload))
			return nil
		},
	)
	consumer.Start(ctx)

	// HTTP server
	server := NewServer(state, hub, publisher, staticDir)
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("[Gateway] Ouvindo HTTP em :%s (static=%s)", port, staticDir)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Gateway] Servidor HTTP morreu: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("[Gateway] Encerrando...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	httpServer.Shutdown(shutdownCtx)
	consumer.Stop()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
