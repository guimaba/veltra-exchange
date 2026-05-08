package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// TransactionKind distingue lancamentos de credito (deposito simulado, sem
// remetente) de transferencias entre contas. Evita ambiguidade quando Sender
// vem vazio.
type TransactionKind string

const (
	KindCredit   TransactionKind = "credit"
	KindTransfer TransactionKind = "transfer"
)

type Transaction struct {
	TxID     string          `json:"tx_id,omitempty"`
	Sender   string          `json:"sender"`
	Receiver string          `json:"receiver"`
	Amount   float64         `json:"amount"`
	Kind     TransactionKind `json:"kind,omitempty"`
}

type Block struct {
	Index        int           `json:"index"`
	Timestamp    int64         `json:"timestamp"`
	Transactions []Transaction `json:"transactions"`
	PrevHash     string        `json:"prev_hash"`
	Hash         string        `json:"hash"`
	Nonce        int           `json:"nonce"`
}

func NewBlock(index int, transactions []Transaction, prevHash string) *Block {
	block := &Block{
		Index:        index,
		Timestamp:    time.Now().Unix(),
		Transactions: transactions,
		PrevHash:     prevHash,
	}
	block.Hash = block.CalculateHash()
	return block
}

func (b *Block) CalculateHash() string {
	record := fmt.Sprintf("%d%d%v%s%d", b.Index, b.Timestamp, b.Transactions, b.PrevHash, b.Nonce)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func (b *Block) Mine(difficulty int) {
	prefix := ""
	for i := 0; i < difficulty; i++ {
		prefix += "0"
	}
	for {
		b.Hash = b.CalculateHash()
		if b.Hash[:difficulty] == prefix {
			break
		}
		b.Nonce++
	}
}
