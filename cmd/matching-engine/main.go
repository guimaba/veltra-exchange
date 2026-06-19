// Comando: matching-engine da Veltra Exchange.
//
// Cluster de replicas com eleicao Bully (failover): apenas o lider consome
// comandos de ordem (order.place/order.cancel) da fila q.matching.commands,
// sequencia, grava no WAL e opera os motores (um por par). Eventos imutaveis
// (order.accepted, trade.executed, order.filled, ...) sao publicados em
// veltra.events para as projecoes (ledger, market data, auditoria).
//
// Configuracao via env vars:
//
//	NODE_ID         - ID numerico da replica (obrigatorio)
//	NODE_PORT       - porta RPC de eleicao (default 9101)
//	PEERS           - "id:host:port,..." das outras replicas
//	AMQP_URL        - URL do RabbitMQ (obrigatorio)
//	PAIRS           - pares suportados, ex.: "VLT/USDT-sim" (default)
//	WAL_DIR         - diretorio do WAL/snapshots (default ./data/matching).
//	                  Em producao, aponte para um volume COMPARTILHADO entre as
//	                  replicas para que a promocao recupere o estado completo.
//	SNAPSHOT_EVERY  - comandos entre snapshots (default 100)
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/bully"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/exchange"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
)

func main() {
	cfg := loadConfig()

	node := bully.NewNode(cfg.ID, cfg.Port, cfg.Peers)
	election := NewElection(node)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ----- RabbitMQ -----
	bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
	client, err := messaging.NewClient(bootCtx, cfg.AMQPURL)
	bootCancel()
	if err != nil {
		log.Fatalf("[Matching %d] Falha ao conectar ao RabbitMQ: %v", cfg.ID, err)
	}
	defer client.Close()

	publisher := messaging.NewPublisher(client)
	defer publisher.Close()

	service := NewMatchingService(cfg.ID, cfg.WALDir, cfg.SnapshotEvery, cfg.Pairs, publisher)

	// ----- Servidor RPC de eleicao -----
	if err := rpc.Register(election); err != nil {
		log.Fatalf("[Matching %d] Erro ao registrar RPC: %v", cfg.ID, err)
	}
	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("[Matching %d] Erro ao ouvir na porta %s: %v", cfg.ID, cfg.Port, err)
	}
	defer ln.Close()
	log.Printf("[Matching %d] Eleicao RPC na porta %s. Pares: %v", cfg.ID, cfg.Port, cfg.Pairs)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go rpc.ServeConn(conn)
		}
	}()

	// Aguarda os pares subirem antes da primeira eleicao.
	time.Sleep(5 * time.Second)
	election.StartElection()

	var consumer *messaging.Consumer
	var consumerMu sync.Mutex

	go monitorLeadership(ctx, cfg.ID, node, client, publisher, service, &consumer, &consumerMu)
	go heartbeatLoop(ctx, cfg.ID, node, election)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	fmt.Printf("[Matching %d] Pronto. Ctrl+C para sair.\n", cfg.ID)
	<-sigCh

	log.Printf("[Matching %d] Encerrando...", cfg.ID)
	cancel()
	consumerMu.Lock()
	if consumer != nil {
		consumer.Stop()
	}
	consumerMu.Unlock()
	service.Deactivate()
}

// monitorLeadership liga/desliga o consumer e (de)ativa os motores conforme a
// lideranca.
func monitorLeadership(
	ctx context.Context,
	id int,
	node *bully.Node,
	client *messaging.Client,
	publisher *messaging.Publisher,
	service *MatchingService,
	consumerRef **messaging.Consumer,
	mu *sync.Mutex,
) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isLeader := node.IsLeader()
			mu.Lock()
			active := *consumerRef != nil
			mu.Unlock()

			if isLeader && !active {
				if err := service.Activate(); err != nil {
					log.Printf("[Matching %d] Falha ao ativar motores: %v", id, err)
					continue
				}
				log.Printf("[Matching %d] Virou lider. Consumindo %s.", id, messaging.QueueMatchingCommands)
				cons := messaging.NewConsumer(client, publisher, messaging.ConsumeOptions{
					Queue:    messaging.QueueMatchingCommands,
					Consumer: fmt.Sprintf("matching-%d", id),
					Prefetch: 1, // serializa -> single-threaded por construcao
				}, service.Handler())
				cons.Start(ctx)
				mu.Lock()
				*consumerRef = cons
				mu.Unlock()
			} else if !isLeader && active {
				log.Printf("[Matching %d] Perdeu lideranca. Parando consumer.", id)
				mu.Lock()
				if *consumerRef != nil {
					(*consumerRef).Stop()
					*consumerRef = nil
				}
				mu.Unlock()
				service.Deactivate()
			}
		}
	}
}

// heartbeatLoop monitora o lider; se cair, dispara nova eleicao.
func heartbeatLoop(ctx context.Context, id int, node *bully.Node, election *Election) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			leaderID := node.GetLeader()
			if leaderID == -1 {
				log.Printf("[Matching %d] Sem lider, iniciando eleicao...", id)
				election.StartElection()
				continue
			}
			if leaderID == id {
				continue
			}
			addr := node.Peers[leaderID]
			if addr == "" {
				continue
			}
			if !strings.Contains(addr, ":") {
				addr = "localhost:" + addr
			}
			c, err := rpc.Dial("tcp", addr)
			if err != nil {
				log.Printf("[Matching %d] Lider %d fora do ar (%v), nova eleicao...", id, leaderID, err)
				node.SetLeader(-1)
				election.StartElection()
				continue
			}
			var reply bool
			callErr := c.Call("Election.Heartbeat", true, &reply)
			c.Close()
			if callErr != nil {
				log.Printf("[Matching %d] Heartbeat falhou (lider %d): %v. Nova eleicao...", id, leaderID, callErr)
				node.SetLeader(-1)
				election.StartElection()
			}
		}
	}
}

// ===== Configuracao =====

type config struct {
	ID            int
	Port          string
	Peers         map[int]string
	AMQPURL       string
	Pairs         []exchange.Pair
	WALDir        string
	SnapshotEvery int
}

func loadConfig() config {
	c := config{
		ID:            envInt("NODE_ID", 0),
		Port:          envStr("NODE_PORT", "9101"),
		Peers:         parsePeers(os.Getenv("PEERS")),
		AMQPURL:       os.Getenv("AMQP_URL"),
		Pairs:         parsePairs(envStr("PAIRS", "VLT/USDT-sim")),
		WALDir:        envStr("WAL_DIR", "./data/matching"),
		SnapshotEvery: envInt("SNAPSHOT_EVERY", 100),
	}
	if c.ID == 0 {
		log.Fatalf("NODE_ID e obrigatorio")
	}
	if c.AMQPURL == "" {
		log.Fatalf("AMQP_URL e obrigatorio")
	}
	if len(c.Pairs) == 0 {
		log.Fatalf("PAIRS nao pode ser vazio")
	}
	return c
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func parsePeers(spec string) map[int]string {
	peers := map[int]string{}
	if spec == "" {
		return peers
	}
	for _, p := range strings.Split(spec, ",") {
		parts := strings.Split(strings.TrimSpace(p), ":")
		switch len(parts) {
		case 2:
			if id, err := strconv.Atoi(parts[0]); err == nil {
				peers[id] = parts[1]
			}
		case 3:
			if id, err := strconv.Atoi(parts[0]); err == nil {
				peers[id] = parts[1] + ":" + parts[2]
			}
		}
	}
	return peers
}

func parsePairs(spec string) []exchange.Pair {
	var pairs []exchange.Pair
	for _, s := range strings.Split(spec, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if p, err := exchange.ParsePair(s); err == nil {
			pairs = append(pairs, p)
		} else {
			log.Printf("Par ignorado (invalido): %q", s)
		}
	}
	return pairs
}
