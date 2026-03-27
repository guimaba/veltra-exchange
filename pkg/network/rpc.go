package network

import (
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/blockchain"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/bully"
	"log"
	"net/rpc"
)

type RPCHandler struct {
	Node       *bully.Node
	Blockchain *blockchain.Blockchain
}

// Métodos RPC do Bully
type ElectionArgs struct {
	FromID int
}

type ElectionReply struct {
	OK bool
}

func (h *RPCHandler) Elect(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Nó %d] Recebeu ELEIÇÃO de %d\n", h.Node.ID, args.FromID)
	reply.OK = true
	// Inicia própria eleição porque temos ID maior que o remetente
	go h.StartElection() 
	return nil
}

func (h *RPCHandler) Coordinator(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Nó %d] Novo Coordenador é %d\n", h.Node.ID, args.FromID)
	h.Node.SetLeader(args.FromID)
	return nil
}

// Métodos RPC da Blockchain
func (h *RPCHandler) ReceiveBlock(block blockchain.Block, reply *bool) error {
	log.Printf("[Nó %d] Recebeu novo bloco #%d\n", h.Node.ID, block.Index)
	err := h.Blockchain.AddBlock(block)
	if err != nil {
		log.Printf("[Nó %d] Erro ao adicionar bloco: %v\n", h.Node.ID, err)
		*reply = false
		return err
	}
	*reply = true
	return nil
}

func (h *RPCHandler) ReceiveTransaction(tx blockchain.Transaction, reply *bool) error {
	log.Printf("[Nó %d] Recebeu transação de %s para %s no valor de %.2f\n", h.Node.ID, tx.Sender, tx.Receiver, tx.Amount)
	
	h.Blockchain.AddTransactionToPool(tx)

	// Se for líder, processa a mineração do novo bloco
	if h.Node.IsLeader() {
		go func() {
			// Usamos uma dificuldade baixa (ex: 3) para permitir testes rápidos locais
			newBlock, err := h.Blockchain.MinePendingTransactions(3)
			if err != nil {
				log.Printf("[Nó %d] Erro na mineração do Líder: %v\n", h.Node.ID, err)
				return
			}
			
			// Propagar bloco minerado para todos os peers da rede
			for peerID, port := range h.Node.Peers {
				if peerID == h.Node.ID {
					continue
				}
				client, err := rpc.Dial("tcp", "localhost:"+port)
				if err != nil {
					continue
				}
				var replyReceive bool
				client.Call("RPCHandler.ReceiveBlock", *newBlock, &replyReceive)
				client.Close()
			}
		}()
	}
	*reply = true
	return nil
}

func (h *RPCHandler) Heartbeat(args bool, reply *bool) error {
	*reply = true
	return nil
}

// StartElection implementa a lógica do Bully
func (h *RPCHandler) StartElection() {
	h.Node.SetState(bully.StateElection)
	higherPeers := h.Node.GetPeersHigherThan(h.Node.ID)
	
	if len(higherPeers) == 0 {
		h.ProclaimCoordinator()
		return
	}

	okReceived := false
	for _, port := range higherPeers {
		client, err := rpc.Dial("tcp", "localhost:"+port)
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
	h.Node.SetLeader(h.Node.ID)
	for peerID, port := range h.Node.Peers {
		if peerID == h.Node.ID {
			continue
		}
		client, err := rpc.Dial("tcp", "localhost:"+port)
		if err != nil {
			continue
		}
		var reply ElectionReply
		client.Call("RPCHandler.Coordinator", ElectionArgs{FromID: h.Node.ID}, &reply)
		client.Close()
	}
}
