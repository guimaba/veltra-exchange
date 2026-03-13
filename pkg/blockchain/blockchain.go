package blockchain

import (
	"errors"
	"sync"
)

type Blockchain struct {
	Blocks []Block
	Mutex  sync.Mutex
}

func NewBlockchain() *Blockchain {
	genesisBlock := NewBlock(0, []Transaction{}, "0")
	return &Blockchain{
		Blocks: []Block{*genesisBlock},
	}
}

func (bc *Blockchain) AddBlock(b Block) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	lastBlock := bc.Blocks[len(bc.Blocks)-1]
	if b.PrevHash != lastBlock.Hash {
		return errors.New("invalid previous hash")
	}

	if b.CalculateHash() != b.Hash {
		return errors.New("invalid hash")
	}

	bc.Blocks = append(bc.Blocks, b)
	return nil
}

func (bc *Blockchain) IsValid() bool {
	for i := 1; i < len(bc.Blocks); i++ {
		current := bc.Blocks[i]
		previous := bc.Blocks[i-1]

		if current.Hash != current.CalculateHash() {
			return false
		}
		if current.PrevHash != previous.Hash {
			return false
		}
	}
	return true
}

func (bc *Blockchain) GetLatestBlock() Block {
	return bc.Blocks[len(bc.Blocks)-1]
}
