package main

import (
	"flag"
	"fmt"
	"github.com/Igor-Schmidt/blockchain_sistemasDistribuidos/pkg/blockchain"
	"github.com/Igor-Schmidt/blockchain_sistemasDistribuidos/pkg/bully"
	"github.com/Igor-Schmidt/blockchain_sistemasDistribuidos/pkg/network"
	"log"
	"net"
	"net/rpc"
	"strconv"
	"strings"
	"time"
)

func main() {
	id := flag.Int("id", 1, "Node ID")
	port := flag.String("port", "8001", "Node Port")
	peersStr := flag.String("peers", "", "Comma separated list of id:port")
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
	bc := blockchain.NewBlockchain()
	handler := &network.RPCHandler{
		Node:       node,
		Blockchain: bc,
	}

	rpc.Register(handler)
	ln, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatalf("Error listening on port %s: %v", *port, err)
	}

	log.Printf("[Node %d] Listening on port %s\n", *id, *port)

	go func() {
		for {
			rpc.Accept(ln)
		}
	}()

	// Initial delay to let all nodes start
	time.Sleep(5 * time.Second)

	// Check leader periodically
	go func() {
		for {
			time.Sleep(10 * time.Second)
			leaderID := node.GetLeader()
			if leaderID == -1 {
				log.Printf("[Node %d] No leader, starting election...\n", *id)
				handler.StartElection()
			} else if leaderID != *id {
				// Check if leader is alive
				port := peers[leaderID]
				client, err := rpc.Dial("tcp", "localhost:"+port)
				if err != nil {
					log.Printf("[Node %d] Leader %d is down, starting election...\n", *id, leaderID)
					node.SetLeader(-1)
					handler.StartElection()
				} else {
					var reply bool
					err = client.Call("RPCHandler.Heartbeat", true, &reply)
					if err != nil {
						log.Printf("[Node %d] Pulse failure for leader %d, starting election...\n", *id, leaderID)
						node.SetLeader(-1)
						handler.StartElection()
					}
					client.Close()
				}
			}
		}
	}()

	// Keep main alive
	fmt.Printf("[Node %d] Running. Press Enter to exit.\n", *id)
	var input string
	fmt.Scanln(&input)
}
