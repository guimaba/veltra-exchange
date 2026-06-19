package main

// Eleicao de lider (Bully) para failover do matching engine — reaproveita o
// pkg/bully do projeto blockchain (plano tecnico secao 3.2: "Eleicao Bully ->
// failover do matching engine"). Apenas o lider consome comandos e opera os
// motores; os demais ficam em standby. A promocao recarrega o estado do WAL
// compartilhado.
//
// Handler RPC auto-contido (so eleicao + heartbeat), independente do
// network.RPCHandler que e acoplado a blockchain.

import (
	"log"
	"net/rpc"
	"strings"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/bully"
)

type ElectionArgs struct {
	FromID int
}

type ElectionReply struct {
	OK bool
}

// Election expoe os metodos RPC do algoritmo Bully sobre um bully.Node.
type Election struct {
	node *bully.Node
}

func NewElection(node *bully.Node) *Election {
	return &Election{node: node}
}

func (e *Election) dialPeer(peerID int) (*rpc.Client, error) {
	addr := e.node.Peers[peerID]
	if addr == "" {
		return nil, rpc.ErrShutdown
	}
	if !strings.Contains(addr, ":") {
		addr = "localhost:" + addr
	}
	return rpc.Dial("tcp", addr)
}

// Elect: um peer de ID menor pediu eleicao; respondemos OK e disparamos a nossa.
func (e *Election) Elect(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Matching %d] Recebeu ELEICAO de %d", e.node.ID, args.FromID)
	reply.OK = true
	go e.StartElection()
	return nil
}

// Coordinator: anuncio de novo lider.
func (e *Election) Coordinator(args ElectionArgs, reply *ElectionReply) error {
	log.Printf("[Matching %d] Novo coordenador e %d", e.node.ID, args.FromID)
	e.node.SetLeader(args.FromID)
	return nil
}

// Heartbeat: usado pelos standbys para verificar se o lider esta vivo.
func (e *Election) Heartbeat(args bool, reply *bool) error {
	*reply = true
	return nil
}

// StartElection inicia o algoritmo Bully: contata peers de ID maior; se nenhum
// responder, se autoproclama coordenador.
func (e *Election) StartElection() {
	e.node.SetState(bully.StateElection)
	higher := e.node.GetPeersHigherThan(e.node.ID)

	if len(higher) == 0 {
		e.ProclaimCoordinator()
		return
	}

	okReceived := false
	for peerID := range higher {
		client, err := e.dialPeer(peerID)
		if err != nil {
			continue
		}
		var reply ElectionReply
		if err := client.Call("Election.Elect", ElectionArgs{FromID: e.node.ID}, &reply); err == nil && reply.OK {
			okReceived = true
		}
		client.Close()
	}

	if !okReceived {
		e.ProclaimCoordinator()
	}
}

// ProclaimCoordinator anuncia este no como lider para todos os peers.
func (e *Election) ProclaimCoordinator() {
	log.Printf("[Matching %d] Eu sou o novo coordenador!", e.node.ID)
	e.node.SetLeader(e.node.ID)
	for peerID := range e.node.Peers {
		if peerID == e.node.ID {
			continue
		}
		client, err := e.dialPeer(peerID)
		if err != nil {
			continue
		}
		var reply ElectionReply
		client.Call("Election.Coordinator", ElectionArgs{FromID: e.node.ID}, &reply)
		client.Close()
	}
}
