package main

import (
	"fmt"
	"log"
	"net/rpc"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/blockchain"
)

func main() {
	ports := []string{"8001", "8002", "8003"}
	var coordinatorPort string

	// Tenta descobrir o coordenador a partir de qualquer nó ativo
	for _, port := range ports {
		client, err := rpc.Dial("tcp", "localhost:"+port)
		if err == nil {
			err = client.Call("RPCHandler.GetCoordinator", true, &coordinatorPort)
			client.Close()
			if err == nil && coordinatorPort != "" {
				fmt.Printf("Coordenador encontrado na porta: %s (informado pelo nó %s)\n", coordinatorPort, port)
				break
			}
		}
	}

	if coordinatorPort == "" {
		log.Fatalf("Erro: Não foi possível encontrar o coordenador na rede.")
	}

	client, err := rpc.Dial("tcp", "localhost:"+coordinatorPort)
	if err != nil {
		log.Fatalf("Erro conectando ao Coordenador na porta %s: %v", coordinatorPort, err)
	}
	defer client.Close()

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

	fmt.Printf("Transação enviada com sucesso ao Coordenador (Porta %s)! Resposta da propagação: %v\n", coordinatorPort, reply)
	fmt.Println("Cheque os logs do coordenador para ver a mineração acontecendo!")
}
