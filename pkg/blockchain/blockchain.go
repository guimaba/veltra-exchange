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
	genesisBlock.Timestamp = 0 // Fixar timestamp para garantir o mesmo Hash em todos os nós
	genesisBlock.Hash = genesisBlock.CalculateHash()
	
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
	
	log.Printf("[Blockchain] Validando bloco #%d...", b.Index)
	
	if b.Index != lastBlock.Index+1 {
		return errors.New("índice do bloco inválido ou fora de ordem")
	}

	if b.PrevHash != lastBlock.Hash {
		return errors.New("hash anterior inválido")
	}

	if b.CalculateHash() != b.Hash {
		return errors.New("hash inválido ou corrompido")
	}

	log.Printf("[Blockchain] Bloco #%d validado com sucesso (Hash: %s).", b.Index, b.Hash)
	bc.Blocks = append(bc.Blocks, b)

	if bc.Storage != nil {
		log.Printf("[Blockchain] Salvando bloco #%d no banco de dados local...", b.Index)
		err := bc.Storage.SaveBlock(b)
		if err != nil {
			log.Printf("[Blockchain] Erro ao salvar bloco no DB: %v\n", err)
			return err
		}
		log.Printf("[Blockchain] Bloco #%d salvo no DB com sucesso.", b.Index)
	}

	// Limpa a MemPool local das transações que foram incluídas no bloco
	// Como simplificação, estamos limpando a mempool inteira
	bc.MemPool = []Transaction{}

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

// GetBalance calcula o saldo de uma conta a partir do ledger:
// soma de creditos recebidos - soma de debitos enviados, considerando tanto
// blocos confirmados quanto transacoes pendentes na MemPool.
//
// Para creditos (Kind=credit, Sender vazio), conta apenas como entrada para o
// Receiver. Para transferencias, debita o Sender e credita o Receiver.
func (bc *Blockchain) GetBalance(account string) float64 {
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	var balance float64
	apply := func(tx Transaction) {
		if tx.Receiver == account {
			balance += tx.Amount
		}
		if tx.Sender == account && tx.Sender != "" {
			balance -= tx.Amount
		}
	}

	for _, b := range bc.Blocks {
		for _, tx := range b.Transactions {
			apply(tx)
		}
	}
	for _, tx := range bc.MemPool {
		apply(tx)
	}
	return balance
}

// HasTransaction retorna true se uma transacao com este TxID ja foi vista
// (em bloco confirmado ou na MemPool). Usado para idempotencia no consumer
// do lider.
func (bc *Blockchain) HasTransaction(txID string) bool {
	if txID == "" {
		return false
	}
	bc.Mutex.Lock()
	defer bc.Mutex.Unlock()

	for _, b := range bc.Blocks {
		for _, tx := range b.Transactions {
			if tx.TxID == txID {
				return true
			}
		}
	}
	for _, tx := range bc.MemPool {
		if tx.TxID == txID {
			return true
		}
	}
	return false
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
	log.Printf("[1. Coordenador] Criando bloco #%d com %d transações e calculando hash (Dificuldade: %d)...\n", newBlock.Index, len(bc.MemPool), difficulty)
	newBlock.Mine(difficulty)
	log.Printf("[1. Coordenador] Bloco #%d criado e hash calculado com sucesso! Hash: %s\n", newBlock.Index, newBlock.Hash)

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

