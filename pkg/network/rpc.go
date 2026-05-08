package network

import (
	"context"
	"log"
	"net/rpc"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/blockchain"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/bully"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
)

// EventEmitter abstrai o publish no Rabbit pra evitar import direto do client.
// O cmd/node injeta uma implementacao que delega ao messaging.Publisher.
type EventEmitter interface {
	EmitEvent(ctx context.Context, routingKey string, schema string, payload any) error
}

type RPCHandler struct {
	Node       *bully.Node
	Blockchain *blockchain.Blockchain
	Emitter    EventEmitter // opcional - se nil, eventos nao sao publicados (modo legado)

	// Difficulty controla o PoW. Default 3 se zero.
	Difficulty int
}

// dialPeer conecta no peer usando o endereco armazenado no Node.Peers.
// Aceita "host:port" (Docker) ou "port" puro (legado simulate.ps1, prefixa localhost).
func (h *RPCHandler) dialPeer(peerID int) (*rpc.Client, error) {
	addr := h.Node.Peers[peerID]
	if addr == "" {
		return nil, rpc.ErrShutdown
	}
	if !containsColon(addr) {
		addr = "localhost:" + addr
	}
	return rpc.Dial("tcp", addr)
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}

// ===== Eleicao Bully =====

type ElectionArgs struct {
	FromID int
}

type ElectionReply struct {
	OK bool
}

func (h *RPCHandler) Elect(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Nó %d] Recebeu ELEIÇÃO de %d\n", h.Node.ID, args.FromID)
	reply.OK = true
	go h.StartElection()
	return nil
}

func (h *RPCHandler) Coordinator(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Nó %d] Novo Coordenador é %d\n", h.Node.ID, args.FromID)
	previous := h.Node.GetLeader()
	h.Node.SetLeader(args.FromID)
	h.publishLeaderChanged(previous, args.FromID, "coordinator_announcement")
	return nil
}

func (h *RPCHandler) StartElection() {
	h.Node.SetState(bully.StateElection)
	higherPeers := h.Node.GetPeersHigherThan(h.Node.ID)

	if len(higherPeers) == 0 {
		h.ProclaimCoordinator()
		return
	}

	okReceived := false
	for peerID := range higherPeers {
		client, err := h.dialPeer(peerID)
		if err != nil {
			continue
		}
		var reply ElectionReply
		err = client.Call("RPCHandler.Elect", ElectionArgs{FromID: h.Node.ID}, &reply)
		if err == nil && reply.OK {
			okReceived = true
		}
		client.Close()
	}

	if !okReceived {
		h.ProclaimCoordinator()
	}
}

func (h *RPCHandler) ProclaimCoordinator() {
	log.Printf("[Nó %d] Eu sou o novo coordenador!\n", h.Node.ID)
	previous := h.Node.GetLeader()
	h.Node.SetLeader(h.Node.ID)
	for peerID := range h.Node.Peers {
		if peerID == h.Node.ID {
			continue
		}
		client, err := h.dialPeer(peerID)
		if err != nil {
			continue
		}
		var reply ElectionReply
		client.Call("RPCHandler.Coordinator", ElectionArgs{FromID: h.Node.ID}, &reply)
		client.Close()
	}
	h.publishLeaderChanged(previous, h.Node.ID, "self_proclaimed")
}

// ===== Blockchain =====

func (h *RPCHandler) ReceiveBlock(block blockchain.Block, reply *bool) error {
	log.Printf("[Nó %d] Recebeu novo bloco #%d\n", h.Node.ID, block.Index)
	if err := h.Blockchain.AddBlock(block); err != nil {
		log.Printf("[Nó %d] Erro ao adicionar bloco: %v\n", h.Node.ID, err)
		*reply = false
		return err
	}
	*reply = true
	return nil
}

// ReceiveTransaction e o caminho legado (simulate.ps1, test_tx.go). Aceita uma
// transacao via RPC, valida saldo se houver Sender, adiciona ao MemPool e
// dispara mineracao se for o lider.
//
// O caminho novo (RabbitMQ) entra via cmd/node consumer e tambem chama
// AddTransactionToPool / triggers de mineracao - mas com mais cuidado de
// idempotencia e publicacao de eventos.
func (h *RPCHandler) ReceiveTransaction(tx blockchain.Transaction, reply *bool) error {
	log.Printf("[Nó %d] Recebeu transação RPC: %s -> %s (%.2f)\n", h.Node.ID, tx.Sender, tx.Receiver, tx.Amount)

	if tx.Kind == "" {
		if tx.Sender == "" {
			tx.Kind = blockchain.KindCredit
		} else {
			tx.Kind = blockchain.KindTransfer
		}
	}

	h.Blockchain.AddTransactionToPool(tx)

	if h.Node.IsLeader() {
		go h.MineAndBroadcast()
	}
	*reply = true
	return nil
}

// MineAndBroadcast minera as transacoes pendentes e propaga o bloco resultante
// para os peers via RPC. Tambem publica o evento block.mined no Rabbit.
// Seguro pra ser chamado de uma goroutine.
func (h *RPCHandler) MineAndBroadcast() {
	difficulty := h.Difficulty
	if difficulty == 0 {
		difficulty = 3
	}
	newBlock, err := h.Blockchain.MinePendingTransactions(difficulty)
	if err != nil {
		log.Printf("[Nó %d] Erro na mineração: %v\n", h.Node.ID, err)
		return
	}

	// Propaga para peers via RPC (consenso interno)
	for peerID := range h.Node.Peers {
		if peerID == h.Node.ID {
			continue
		}
		client, err := h.dialPeer(peerID)
		if err != nil {
			continue
		}
		var ok bool
		client.Call("RPCHandler.ReceiveBlock", *newBlock, &ok)
		client.Close()
	}

	// Publica evento no Rabbit (notificacao para gateway/auditoria)
	h.publishBlockMined(newBlock)
}

func (h *RPCHandler) Heartbeat(args bool, reply *bool) error {
	*reply = true
	return nil
}

// ===== Helpers de publicacao =====

func (h *RPCHandler) publishBlockMined(b *blockchain.Block) {
	if h.Emitter == nil {
		return
	}
	txs := make([]messaging.BlockMinedTransaction, 0, len(b.Transactions))
	for _, t := range b.Transactions {
		txs = append(txs, messaging.BlockMinedTransaction{
			TxID:     t.TxID,
			Sender:   t.Sender,
			Receiver: t.Receiver,
			Amount:   t.Amount,
			Kind:     string(t.Kind),
		})
	}
	payload := messaging.BlockMinedPayload{
		Index:        b.Index,
		PrevHash:     b.PrevHash,
		Hash:         b.Hash,
		Nonce:        b.Nonce,
		Transactions: txs,
		MinerNodeID:  h.Node.ID,
	}
	if err := h.Emitter.EmitEvent(context.Background(), messaging.RKBlockMined, messaging.SchemaBlockMined, payload); err != nil {
		log.Printf("[Nó %d] Falha ao publicar block.mined: %v", h.Node.ID, err)
	}
}

func (h *RPCHandler) publishLeaderChanged(previous, current int, reason string) {
	if h.Emitter == nil || previous == current {
		return
	}
	payload := messaging.LeaderChangedPayload{
		PreviousLeader: previous,
		NewLeader:      current,
		Reason:         reason,
	}
	if err := h.Emitter.EmitEvent(context.Background(), messaging.RKLeaderChanged, messaging.SchemaLeaderChanged, payload); err != nil {
		log.Printf("[Nó %d] Falha ao publicar leader.changed: %v", h.Node.ID, err)
	}
}
