package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strconv"
	"strings"
	"time"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/blockchain"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/bully"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/database"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/network"
)

func main() {
	id := flag.Int("id", 1, "ID do Nó")
	port := flag.String("port", "8001", "Porta do Nó")
	peersStr := flag.String("peers", "", "Lista de id:porta separada por vírgulas")
	flag.Parse()

	peers := make(map[int]string)
	if *peersStr != "" {
		for _, p := range strings.Split(*peersStr, ",") {
			parts := strings.Split(p, ":")
			if len(parts) == 2 {
				pID, _ := strconv.Atoi(parts[0])
				peers[pID] = parts[1]
			}
		}
	}

	node := bully.NewNode(*id, *port, peers)

	db, err := database.NewMariaDB(*id, "root", "123456", "127.0.0.1", "3306")
	if err != nil {
		log.Printf("[Nó %d] Aviso: falha ao conectar ao MariaDB: %v. Rodando apenas em memória.\n", *id, err)
		db = nil
	} else {
		defer db.Close()
	}

	var storage blockchain.Storage
	if db != nil {
		storage = db
	}

	bc := blockchain.NewBlockchain(storage)
	handler := &network.RPCHandler{
		Node:       node,
		Blockchain: bc,
	}

	rpc.Register(handler)
	ln, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatalf("Erro ao ouvir na porta %s: %v", *port, err)
	}

	log.Printf("[Nó %d] Ouvindo na porta %s\n", *id, *port)

	go func() {
		for {
			rpc.Accept(ln)
		}
	}()

	// Atraso inicial para permitir que todos os nós iniciem
	time.Sleep(5 * time.Second)

	// Verifica o líder periodicamente
	go func() {
		for {
			time.Sleep(10 * time.Second)
			leaderID := node.GetLeader()
			if leaderID == -1 {
				log.Printf("[Nó %d] Sem líder, iniciando eleição...\n", *id)
				handler.StartElection()
			} else if leaderID != *id {
				// Verifica se o líder está vivo
				port := peers[leaderID]
				client, err := rpc.Dial("tcp", "localhost:"+port)
				if err != nil {
					log.Printf("[Nó %d] Líder %d está fora do ar, iniciando eleição...\n", *id, leaderID)
					node.SetLeader(-1)
					handler.StartElection()
				} else {
					var reply bool
					err = client.Call("RPCHandler.Heartbeat", true, &reply)
					if err != nil {
						log.Printf("[Nó %d] Falha de pulso para o líder %d, iniciando eleição...\n", *id, leaderID)
						node.SetLeader(-1)
						handler.StartElection()
					}
					client.Close()
				}
			}
		}
	}()

	// Mantém o programa principal ativo
	fmt.Printf("[Nó %d] Rodando. Pressione Enter para sair.\n", *id)
	var input string
	fmt.Scanln(&input)
}
