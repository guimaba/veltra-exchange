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
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/pgstore"
)

func main() {
	port := envOr("GATEWAY_PORT", "8080")
	amqpURL := os.Getenv("AMQP_URL")
	pgDSN := os.Getenv("POSTGRES_DSN")
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

	// Auth (Postgres) — opcional: se POSTGRES_DSN não estiver set, auth fica desabilitada
	var authServer *AuthServer
	if pgDSN != "" {
		pgDB, err := pgstore.NewDB(pgDSN)
		if err != nil {
			log.Printf("[Gateway] Aviso: falha ao conectar ao Postgres (%v) — auth desabilitada", err)
		} else {
			authServer = NewAuthServer(pgDB)
			authServer.SeedDefaultAdmin(ctx)
			log.Printf("[Gateway] Auth habilitada (Postgres conectado)")
		}
	} else {
		log.Printf("[Gateway] POSTGRES_DSN não configurado — auth desabilitada")
	}

	// Estado + Hub
	state := NewState()
	veltra := NewVeltraState()
	hub := NewHub()
	go hub.Run()

	// HTTP server (criado antes dos consumers para que o consumer da Veltra
	// possa liberar holds do OMS via server.ledger em eventos terminais).
	server := NewServer(state, veltra, hub, publisher, authServer, staticDir)

	// Consumer dos eventos da blockchain -> atualiza state + hub
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

	// Consumer dos eventos da Veltra Exchange -> projecoes de market data + hub.
	// (Quando o servico dedicado de market data existir, esta projecao migra
	// para la; o gateway segue sendo a borda WS — plano secao 4.1/4.4.)
	veltraConsumer := messaging.NewConsumer(
		client,
		publisher,
		messaging.ConsumeOptions{
			Queue:    messaging.QueueMarketDataEvents,
			Consumer: "gateway-veltra",
			Prefetch: 50,
		},
		func(ctx context.Context, env messaging.Envelope, d amqp.Delivery) error {
			veltra.ApplyEvent(d.RoutingKey, env)
			// OMS: libera a reserva quando a ordem fecha (filled/canceled/rejected).
			releaseHoldOnTerminal(ctx, server.ledger, veltra, d.RoutingKey, env)
			hub.PushEvent(d.RoutingKey, json.RawMessage(env.Payload))
			return nil
		},
	)
	veltraConsumer.Start(ctx)

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
	veltraConsumer.Stop()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
