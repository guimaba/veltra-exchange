package bully

import (
	"fmt"
	"sync"
)

type NodeState string

const (
	StateRunning  NodeState = "RUNNING"
	StateElection NodeState = "ELECTION"
	StateLeader   NodeState = "LEADER"
)

type Node struct {
	ID            int
	Port          string
	Peers         map[int]string // ID -> Port
	LeaderID      int
	State         NodeState
	Mutex         sync.RWMutex
}

func NewNode(id int, port string, peers map[int]string) *Node {
	return &Node{
		ID:       id,
		Port:     port,
		Peers:    peers,
		LeaderID: -1,
		State:    StateRunning,
	}
}

func (n *Node) SetLeader(leaderID int) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	n.LeaderID = leaderID
	if leaderID == n.ID {
		n.State = StateLeader
	} else {
		n.State = StateRunning
	}
}

func (n *Node) GetLeader() int {
	n.Mutex.RLock()
	defer n.Mutex.RUnlock()
	return n.LeaderID
}

func (n *Node) IsLeader() bool {
	n.Mutex.RLock()
	defer n.Mutex.RUnlock()
	return n.ID == n.LeaderID
}

func (n *Node) SetState(state NodeState) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	n.State = state
}

func (n *Node) GetPeersHigherThan(id int) map[int]string {
	higher := make(map[int]string)
	for peerID, port := range n.Peers {
		if peerID > id {
			higher[peerID] = port
		}
	}
	return higher
}

func (n *Node) String() string {
	return fmt.Sprintf("Node{ID: %d, Port: %s, Leader: %d, State: %s}", n.ID, n.Port, n.LeaderID, n.State)
}
