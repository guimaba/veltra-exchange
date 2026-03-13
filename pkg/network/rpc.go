package network

import (
	"github.com/Igor-Schmidt/blockchain_sistemasDistribuidos/pkg/blockchain"
	"github.com/Igor-Schmidt/blockchain_sistemasDistribuidos/pkg/bully"
	"log"
	"net/rpc"
)

type RPCHandler struct {
	Node       *bully.Node
	Blockchain *blockchain.Blockchain
}

// Bully RPC Methods
type ElectionArgs struct {
	FromID int
}

type ElectionReply struct {
	OK bool
}

func (h *RPCHandler) Elect(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Node %d] Received ELECTION from %d\n", h.Node.ID, args.FromID)
	reply.OK = true
	// Start own election because we have a higher ID than the sender
	go h.StartElection() 
	return nil
}

func (h *RPCHandler) Coordinator(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Node %d] New Coordinator is %d\n", h.Node.ID, args.FromID)
	h.Node.SetLeader(args.FromID)
	return nil
}

// Blockchain RPC Methods
func (h *RPCHandler) ReceiveBlock(block blockchain.Block, reply *bool) error {
	log.Printf("[Node %d] Received new block #%d\n", h.Node.ID, block.Index)
	err := h.Blockchain.AddBlock(block)
	if err != nil {
		log.Printf("[Node %d] Error adding block: %v\n", h.Node.ID, err)
		*reply = false
		return err
	}
	*reply = true
	return nil
}

func (h *RPCHandler) ReceiveTransaction(tx blockchain.Transaction, reply *bool) error {
	log.Printf("[Node %d] Received transaction from %s to %s\n", h.Node.ID, tx.Sender, tx.Receiver)
	// If leader, should mine a block. For MVP, we'll just log it.
	if h.Node.IsLeader() {
		// Mine logic would go here
	}
	*reply = true
	return nil
}

func (h *RPCHandler) Heartbeat(args bool, reply *bool) error {
	*reply = true
	return nil
}

// StartElection implements the Bully logic
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
	log.Printf("[Node %d] I am the new coordinator!\n", h.Node.ID)
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
