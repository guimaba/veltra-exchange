package blockchain

import (
	"errors"
	"log"
	"sync"
)

type Storage interface {
	SaveBlock(b Block) error
	LoadBlocks() ([]Block, error)
}

type Blockchain struct {
	Blocks  []Block
	Mutex   sync.Mutex
	Storage Storage
}

func NewBlockchain(storage Storage) *Blockchain {
	bc := &Blockchain{
		Storage: storage,
	}

	if storage != nil {
		blocks, err := storage.LoadBlocks()
		if err == nil && len(blocks) > 0 {
			log.Printf("[Blockchain] Carregados %d blocos existentes do DB\n", len(blocks))
			bc.Blocks = blocks
			return bc
		}
	}

	genesisBlock := NewBlock(0, []Transaction{}, "0")
	bc.Blocks = []Block{*genesisBlock}

	if storage != nil {
		storage.SaveBlock(*genesisBlock)
	}

	return bc
}

func (bc *Blockchain) AddBlock(b Block) error {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	lastBlock := bc.Blocks[len(bc.Blocks)-1]
	if b.PrevHash != lastBlock.Hash {
		return errors.New("hash anterior inválido")
	}

	if b.CalculateHash() != b.Hash {
		return errors.New("hash inválido")
	}

	bc.Blocks = append(bc.Blocks, b)

	if bc.Storage != nil {
		err := bc.Storage.SaveBlock(b)
		if err != nil {
			log.Printf("[Blockchain] Erro ao salvar bloco no DB: %v\n", err)
			return err
		}
	}

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
