package main

import (
	"fmt"
	"log"
	"net/rpc"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/blockchain"
)

func main() {
	// No simulate.ps1, o Nó 3 (porta 8003) geralmente se torna o líder rapidamente devido ao maior ID
	client, err := rpc.Dial("tcp", "localhost:8003")
	if err != nil {
		log.Fatalf("Erro conectando ao Nó da porta 8003: %v\nTentando conectar a outro? Altere a porta no test_tx.go", err)
	}

	tx := blockchain.Transaction{
		Sender:   "Alice",
		Receiver: "Bob",
		Amount:   50.50,
	}

	var reply bool
	err = client.Call("RPCHandler.ReceiveTransaction", tx, &reply)
	if err != nil {
		log.Fatalf("Erro ao enviar transação: %v", err)
	}

	fmt.Printf("Transação enviada com sucesso ao Nó! Resposta da propagação: %v\n", reply)
	fmt.Println("Cheque os logs dos terminais (especialmente do Nó 3) para ver a mineração acontecendo!")
}
