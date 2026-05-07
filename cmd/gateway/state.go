package main

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
)

// State e o snapshot em memoria mantido pelo gateway, alimentado pelos eventos
// vindos do Rabbit. Serve a UI:
//   - saldo por conta (atualizado a partir de credit.added e block.mined)
//   - lista de blocos recentes (ultimos N)
//   - lider atual
//
// Se o gateway reiniciar, o snapshot zera; usuario sera reabastecido conforme
// novos eventos chegam. Para um dashboard mais robusto, persistir em DB ou
// "rebuild" a partir do audit_events seria a evolucao natural.
type State struct {
	mu sync.RWMutex

	balances       map[string]float64
	recentBlocks   []messaging.BlockMinedPayload
	maxBlocks      int
	currentLeader  int
	pendingTxIDs   map[string]bool // tx em flight (publicada, aguardando resposta)
}

func NewState() *State {
	return &State{
		balances:     map[string]float64{},
		recentBlocks: []messaging.BlockMinedPayload{},
		maxBlocks:    50,
		currentLeader: -1,
		pendingTxIDs: map[string]bool{},
	}
}

// ApplyEvent atualiza o snapshot a partir de um envelope vindo de q.gateway.events.
// Idempotente: aceita mensagens duplicadas sem corromper o estado.
func (s *State) ApplyEvent(routingKey string, env messaging.Envelope) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch routingKey {
	case messaging.RKCreditAdded:
		var p messaging.CreditAddedPayload
		if err := env.Unmarshal(&p); err == nil {
			// new_balance vem do lider e ja eh autoritativo.
			s.balances[p.Account] = p.NewBalance
		}
	case messaging.RKTransactionReceived:
		var p messaging.TransactionReceivedPayload
		if err := env.Unmarshal(&p); err == nil {
			for acc, bal := range p.BalanceAfter {
				s.balances[acc] = bal
			}
			delete(s.pendingTxIDs, env.TxID)
		}
	case messaging.RKTransactionRejected:
		delete(s.pendingTxIDs, env.TxID)
	case messaging.RKBlockMined:
		var p messaging.BlockMinedPayload
		if err := env.Unmarshal(&p); err == nil {
			s.appendBlock(p)
		}
	case messaging.RKLeaderChanged:
		var p messaging.LeaderChangedPayload
		if err := env.Unmarshal(&p); err == nil {
			s.currentLeader = p.NewLeader
		}
	}
}

func (s *State) appendBlock(b messaging.BlockMinedPayload) {
	// Evita duplicar (mesmo block.index ja presente)
	for _, existing := range s.recentBlocks {
		if existing.Index == b.Index && existing.Hash == b.Hash {
			return
		}
	}
	s.recentBlocks = append(s.recentBlocks, b)
	if len(s.recentBlocks) > s.maxBlocks {
		s.recentBlocks = s.recentBlocks[len(s.recentBlocks)-s.maxBlocks:]
	}
}

func (s *State) Balance(account string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.balances[account]
}

func (s *State) AllBalances() map[string]float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]float64, len(s.balances))
	for k, v := range s.balances {
		out[k] = v
	}
	return out
}

func (s *State) RecentBlocks() []messaging.BlockMinedPayload {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]messaging.BlockMinedPayload, len(s.recentBlocks))
	copy(out, s.recentBlocks)
	return out
}

func (s *State) Leader() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentLeader
}

func (s *State) MarkPending(txID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingTxIDs[txID] = true
}

// SnapshotJSON retorna um snapshot serializado (usado em GET /api/state).
func (s *State) SnapshotJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.Marshal(struct {
		Balances     map[string]float64               `json:"balances"`
		RecentBlocks []messaging.BlockMinedPayload    `json:"recent_blocks"`
		Leader       int                              `json:"leader"`
		Timestamp    time.Time                        `json:"timestamp"`
	}{
		Balances:     s.balances,
		RecentBlocks: s.recentBlocks,
		Leader:       s.currentLeader,
		Timestamp:    time.Now().UTC(),
	})
}
