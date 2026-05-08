// Comando: no da blockchain.
//
// Configuracao via env vars (com fallback pra flags, pra preservar simulate.ps1):
//
//	NODE_ID    - ID numerico do no
//	NODE_PORT  - porta RPC (default 8001)
//	PEERS      - "id:host:port,id:host:port,..." (Docker) ou "id:port,..." (legado)
//	DB_HOST, DB_PORT, DB_USER, DB_PASS - conexao MariaDB (opcional)
//	AMQP_URL   - URL do RabbitMQ. Se vazio, mensageria e desabilitada.
package main

import (
	"context"
	"flag"
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

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/blockchain"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/bully"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/database"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/network"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	cfg := loadConfig()

	// ----- Bully + Blockchain -----
	node := bully.NewNode(cfg.ID, cfg.Port, cfg.Peers)

	var storage blockchain.Storage
	if cfg.DBHost != "" {
		db, err := database.NewMariaDB(cfg.ID, cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort)
		if err != nil {
			log.Printf("[Nó %d] Aviso: falha ao conectar ao MariaDB: %v. Rodando apenas em memoria.", cfg.ID, err)
		} else {
			defer db.Close()
			storage = db
		}
	}
	bc := blockchain.NewBlockchain(storage)

	handler := &network.RPCHandler{
		Node:       node,
		Blockchain: bc,
		Difficulty: 3,
	}

	// ----- Mensageria (opcional) -----
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var publisher *messaging.Publisher
	var leaderConsumer *messaging.Consumer
	var leaderConsumerMu sync.Mutex

	if cfg.AMQPURL != "" {
		bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
		client, err := messaging.NewClient(bootCtx, cfg.AMQPURL)
		bootCancel()
		if err != nil {
			log.Fatalf("[Nó %d] Falha ao conectar ao RabbitMQ: %v", cfg.ID, err)
		}
		defer client.Close()

		publisher = messaging.NewPublisher(client)
		defer publisher.Close()

		handler.Emitter = &emitter{publisher: publisher}

		// Goroutine que monitora estado de lider e starta/stopa o consumer.
		go monitorLeadership(ctx, cfg.ID, node, client, publisher, bc, handler, &leaderConsumer, &leaderConsumerMu)
	} else {
		log.Printf("[Nó %d] AMQP_URL nao configurada - mensageria desabilitada.", cfg.ID)
	}

	// ----- Servidor RPC -----
	if err := rpc.Register(handler); err != nil {
		log.Fatalf("[Nó %d] Erro ao registrar RPC: %v", cfg.ID, err)
	}
	ln, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		log.Fatalf("[Nó %d] Erro ao ouvir na porta %s: %v", cfg.ID, cfg.Port, err)
	}
	defer ln.Close()
	log.Printf("[Nó %d] Ouvindo RPC na porta %s", cfg.ID, cfg.Port)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go rpc.ServeConn(conn)
		}
	}()

	// Atraso pra permitir que todos os pares iniciem antes da primeira eleicao.
	time.Sleep(5 * time.Second)

	// Loop de monitoramento do lider via heartbeat.
	go heartbeatLoop(ctx, cfg.ID, node, handler)

	// Aguarda sinal de termino.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("[Nó %d] Pronto. Ctrl+C para sair.\n", cfg.ID)
	<-sigCh
	log.Printf("[Nó %d] Encerrando...", cfg.ID)
	cancel()
	leaderConsumerMu.Lock()
	if leaderConsumer != nil {
		leaderConsumer.Stop()
	}
	leaderConsumerMu.Unlock()
}

// ===== Configuracao =====

type config struct {
	ID                       int
	Port                     string
	Peers                    map[int]string
	DBUser, DBPass           string
	DBHost, DBPort           string
	AMQPURL                  string
}

func loadConfig() config {
	// Flags como fallback compatibilidade simulate.ps1 / test_tx.go
	idFlag := flag.Int("id", 0, "ID do no")
	portFlag := flag.String("port", "", "Porta RPC")
	peersFlag := flag.String("peers", "", "Lista de peers")
	flag.Parse()

	c := config{
		ID:      envInt("NODE_ID", *idFlag),
		Port:    envStr("NODE_PORT", *portFlag),
		Peers:   parsePeers(envStr("PEERS", *peersFlag)),
		DBHost:  os.Getenv("DB_HOST"),
		DBPort:  envStr("DB_PORT", "3306"),
		DBUser:  os.Getenv("DB_USER"),
		DBPass:  os.Getenv("DB_PASS"),
		AMQPURL: os.Getenv("AMQP_URL"),
	}

	// Compatibilidade: aceita DB_DSN no formato MySQL
	// (user:pass@tcp(host:port)/) e extrai os campos
	if dsn := os.Getenv("DB_DSN"); dsn != "" && c.DBHost == "" {
		if u, p, h, port := parseMySQLDSN(dsn); u != "" {
			c.DBUser, c.DBPass, c.DBHost, c.DBPort = u, p, h, port
		}
	}

	if c.ID == 0 {
		log.Fatalf("NODE_ID (ou -id) e obrigatorio")
	}
	if c.Port == "" {
		c.Port = "8001"
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

// parsePeers aceita "id:host:port" (3 partes) ou "id:port" (2 partes, legado).
// Armazena valor como "host:port" pronto pra rpc.Dial.
func parsePeers(spec string) map[int]string {
	peers := map[int]string{}
	if spec == "" {
		return peers
	}
	for _, p := range strings.Split(spec, ",") {
		parts := strings.Split(strings.TrimSpace(p), ":")
		switch len(parts) {
		case 2:
			id, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			peers[id] = parts[1] // formato legado: so porta
		case 3:
			id, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}
			peers[id] = parts[1] + ":" + parts[2]
		}
	}
	return peers
}

// parseMySQLDSN extrai user/pass/host/port de um DSN no formato
// "user:pass@tcp(host:port)/db?params". Implementacao simples - bom o
// suficiente pro nosso caso de uso.
func parseMySQLDSN(dsn string) (user, pass, host, port string) {
	atIdx := strings.Index(dsn, "@tcp(")
	if atIdx == -1 {
		return
	}
	credPart := dsn[:atIdx]
	rest := dsn[atIdx+5:]

	if i := strings.Index(credPart, ":"); i >= 0 {
		user = credPart[:i]
		pass = credPart[i+1:]
	} else {
		user = credPart
	}

	closeIdx := strings.Index(rest, ")")
	if closeIdx == -1 {
		return
	}
	addr := rest[:closeIdx]
	if i := strings.Index(addr, ":"); i >= 0 {
		host = addr[:i]
		port = addr[i+1:]
	} else {
		host = addr
	}
	return
}

// ===== Loop de heartbeat =====

func heartbeatLoop(ctx context.Context, id int, node *bully.Node, handler *network.RPCHandler) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			leaderID := node.GetLeader()
			if leaderID == -1 {
				log.Printf("[Nó %d] Sem lider, iniciando eleicao...", id)
				handler.StartElection()
				continue
			}
			if leaderID == id {
				continue
			}
			// Verifica se o lider esta vivo
			addr := node.Peers[leaderID]
			if addr == "" {
				continue
			}
			if !strings.Contains(addr, ":") {
				addr = "localhost:" + addr
			}
			client, err := rpc.Dial("tcp", addr)
			if err != nil {
				log.Printf("[Nó %d] Lider %d fora do ar (%s), iniciando eleicao...", id, leaderID, err)
				node.SetLeader(-1)
				handler.StartElection()
				continue
			}
			var reply bool
			callErr := client.Call("RPCHandler.Heartbeat", true, &reply)
			client.Close()
			if callErr != nil {
				log.Printf("[Nó %d] Heartbeat falhou para lider %d: %v. Iniciando eleicao...", id, leaderID, callErr)
				node.SetLeader(-1)
				handler.StartElection()
			}
		}
	}
}

// ===== Monitoramento de lideranca - liga/desliga consumer de comandos =====

func monitorLeadership(
	ctx context.Context,
	id int,
	node *bully.Node,
	client *messaging.Client,
	publisher *messaging.Publisher,
	bc *blockchain.Blockchain,
	handler *network.RPCHandler,
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
				log.Printf("[Nó %d] Virou lider. Iniciando consumer de comandos.", id)
				cons := messaging.NewConsumer(
					client,
					publisher,
					messaging.ConsumeOptions{
						Queue:    messaging.QueueLeaderCommands,
						Consumer: fmt.Sprintf("leader-%d", id),
						Prefetch: 1,
					},
					leaderHandler(id, bc, handler, publisher),
				)
				cons.Start(ctx)
				mu.Lock()
				*consumerRef = cons
				mu.Unlock()
			} else if !isLeader && active {
				log.Printf("[Nó %d] Perdeu lideranca. Parando consumer de comandos.", id)
				mu.Lock()
				if *consumerRef != nil {
					(*consumerRef).Stop()
					*consumerRef = nil
				}
				mu.Unlock()
			}
		}
	}
}

// ===== emitter =====

// emitter implementa network.EventEmitter delegando ao publisher.
type emitter struct {
	publisher *messaging.Publisher
}

func (e *emitter) EmitEvent(ctx context.Context, routingKey, schema string, payload any) error {
	env, err := messaging.NewEnvelope(schema, "", payload)
	if err != nil {
		return err
	}
	return e.publisher.Publish(ctx, messaging.ExchangeEvents, routingKey, env, nil)
}

// ===== Handler dos comandos consumidos pelo lider =====

// leaderHandler retorna o Handler que processa credit.requested e
// transaction.requested. Toda a logica de validacao de saldo e idempotencia
// vive aqui.
func leaderHandler(nodeID int, bc *blockchain.Blockchain, handler *network.RPCHandler, pub *messaging.Publisher) messaging.Handler {
	return func(ctx context.Context, env messaging.Envelope, d amqp.Delivery) error {
		// Idempotencia: se ja temos essa tx, ignora.
		if bc.HasTransaction(env.TxID) {
			log.Printf("[Nó %d] Tx %s ja processada. Skip.", nodeID, env.TxID)
			return nil
		}

		switch d.RoutingKey {
		case messaging.RKCreditRequested:
			return processCredit(ctx, nodeID, env, bc, handler, pub)
		case messaging.RKTransactionRequested:
			return processTransaction(ctx, nodeID, env, bc, handler, pub)
		default:
			return messaging.Permanent("routing key desconhecida: "+d.RoutingKey, nil)
		}
	}
}

func processCredit(ctx context.Context, nodeID int, env messaging.Envelope, bc *blockchain.Blockchain, handler *network.RPCHandler, pub *messaging.Publisher) error {
	var p messaging.CreditRequestedPayload
	if err := env.Unmarshal(&p); err != nil {
		return messaging.Permanent("payload de credit invalido", err)
	}
	if p.Account == "" || p.Amount <= 0 {
		return messaging.Business("credit invalido: account vazia ou amount <= 0")
	}

	tx := blockchain.Transaction{
		TxID:     env.TxID,
		Sender:   "",
		Receiver: p.Account,
		Amount:   p.Amount,
		Kind:     blockchain.KindCredit,
	}
	bc.AddTransactionToPool(tx)

	// Publica credit.added imediatamente; mineracao acontece em background.
	newBalance := bc.GetBalance(p.Account)
	out := messaging.CreditAddedPayload{Account: p.Account, Amount: p.Amount, NewBalance: newBalance}
	outEnv, _ := messaging.NewEnvelope(messaging.SchemaCreditAdded, env.TxID, out)
	if err := pub.Publish(ctx, messaging.ExchangeEvents, messaging.RKCreditAdded, outEnv, nil); err != nil {
		log.Printf("[Nó %d] Falha ao publicar credit.added: %v", nodeID, err)
	}

	go handler.MineAndBroadcast()
	return nil
}

func processTransaction(ctx context.Context, nodeID int, env messaging.Envelope, bc *blockchain.Blockchain, handler *network.RPCHandler, pub *messaging.Publisher) error {
	var p messaging.TransactionRequestedPayload
	if err := env.Unmarshal(&p); err != nil {
		return messaging.Permanent("payload de transaction invalido", err)
	}
	if p.Sender == "" || p.Receiver == "" || p.Amount <= 0 {
		return messaging.Business("transaction invalida: sender/receiver/amount obrigatorios")
	}

	balance := bc.GetBalance(p.Sender)
	if balance < p.Amount {
		// Erro de negocio: publica transaction.rejected, ACK.
		out := messaging.TransactionRejectedPayload{
			Sender:         p.Sender,
			Receiver:       p.Receiver,
			Amount:         p.Amount,
			Reason:         "INSUFFICIENT_FUNDS",
			CurrentBalance: balance,
		}
		outEnv, _ := messaging.NewEnvelope(messaging.SchemaTransactionRejected, env.TxID, out)
		if err := pub.Publish(ctx, messaging.ExchangeEvents, messaging.RKTransactionRejected, outEnv, nil); err != nil {
			log.Printf("[Nó %d] Falha ao publicar transaction.rejected: %v", nodeID, err)
		}
		return nil
	}

	tx := blockchain.Transaction{
		TxID:     env.TxID,
		Sender:   p.Sender,
		Receiver: p.Receiver,
		Amount:   p.Amount,
		Kind:     blockchain.KindTransfer,
	}
	bc.AddTransactionToPool(tx)

	out := messaging.TransactionReceivedPayload{
		Sender:   p.Sender,
		Receiver: p.Receiver,
		Amount:   p.Amount,
		BalanceAfter: map[string]float64{
			p.Sender:   bc.GetBalance(p.Sender),
			p.Receiver: bc.GetBalance(p.Receiver),
		},
	}
	outEnv, _ := messaging.NewEnvelope(messaging.SchemaTransactionReceived, env.TxID, out)
	if err := pub.Publish(ctx, messaging.ExchangeEvents, messaging.RKTransactionReceived, outEnv, nil); err != nil {
		log.Printf("[Nó %d] Falha ao publicar transaction.received: %v", nodeID, err)
	}

	go handler.MineAndBroadcast()
	return nil
}
