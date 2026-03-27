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
	MemPool []Transaction
	Mutex   sync.Mutex
	Storage Storage
}

func NewBlockchain(storage Storage) *Blockchain {
	bc := &Blockchain{
		Storage: storage,
		MemPool: []Transaction{},
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

func (bc *Blockchain) AddTransactionToPool(tx Transaction) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()
	bc.MemPool = append(bc.MemPool, tx)
	log.Printf("[Blockchain] Transação adicionada à MemPool local. Total pendente: %d\n", len(bc.MemPool))
}

func (bc *Blockchain) MinePendingTransactions(difficulty int) (*Block, error) {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	if len(bc.MemPool) == 0 {
		return nil, errors.New("nenhuma transação pendente para minerar")
	}

	lastBlock := bc.Blocks[len(bc.Blocks)-1]

	// Cria o novo bloco com as transações pendentes
	newBlock := NewBlock(lastBlock.Index+1, bc.MemPool, lastBlock.Hash)

	// Inicia a Prova de Trabalho
	log.Printf("[Blockchain] Iniciando mineração do bloco %d com %d transações (Dificuldade: %d)...\n", newBlock.Index, len(bc.MemPool), difficulty)
	newBlock.Mine(difficulty)
	log.Printf("[Blockchain] Bloco %d minerado com sucesso! Hash: %s\n", newBlock.Index, newBlock.Hash)

	// Valida integridade do hash do bloco anterior
	if newBlock.PrevHash != lastBlock.Hash {
		return nil, errors.New("hash anterior inválido na mineração")
	}

	// Adiciona à corrente local
	bc.Blocks = append(bc.Blocks, *newBlock)

	// Persiste no Banco de Dados
	if bc.Storage != nil {
		err := bc.Storage.SaveBlock(*newBlock)
		if err != nil {
			log.Printf("[Blockchain] Erro ao salvar bloco minerado no DB: %v\n", err)
			return nil, err
		}
	}

	// Limpa a MemPool após a mineração bem-sucedida
	bc.MemPool = []Transaction{}

	return newBlock, nil
}

