package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/blockchain"
	_ "github.com/go-sql-driver/mysql"
)

type MariaDB struct {
	Conn   *sql.DB
	NodeID int
}

func NewMariaDB(nodeID int, dbUser, dbPass, dbHost, dbPort string) (*MariaDB, error) {
	// 1. Connect without specifying the DB to create it if it doesn't exist
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", dbUser, dbPass, dbHost, dbPort)
	if dbPass == "" {
		dsn = fmt.Sprintf("%s@tcp(%s:%s)/", dbUser, dbHost, dbPort)
	}

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar ao mariadb: %w", err)
	}

	dbName := fmt.Sprintf("blockchain_node_%d", nodeID)
	_, err = conn.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s;", dbName))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar banco de dados %s: %w", dbName, err)
	}
	conn.Close()

	// 2. Conecta ao banco de dados específico do nó
	dsnDB := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName)
	if dbPass == "" {
		dsnDB = fmt.Sprintf("%s@tcp(%s:%s)/%s", dbUser, dbHost, dbPort, dbName)
	}
	connDB, err := sql.Open("mysql", dsnDB)
	if err != nil {
		return nil, fmt.Errorf("falha ao conectar ao banco de dados do nó: %w", err)
	}

	db := &MariaDB{Conn: connDB, NodeID: nodeID}
	
	// 3. Inicializa as tabelas
	err = db.InitTables()
	if err != nil {
		return nil, fmt.Errorf("falha ao inicializar tabelas: %w", err)
	}

	log.Printf("[Banco de Dados] Conectado com sucesso ao MariaDB (%s)\n", dbName)
	return db, nil
}

func (db *MariaDB) InitTables() error {
	createBlocksTable := `
	CREATE TABLE IF NOT EXISTS blocks (
		id INT AUTO_INCREMENT PRIMARY KEY,
		block_index INT NOT NULL,
		timestamp BIGINT NOT NULL,
		prev_hash VARCHAR(255) NOT NULL,
		hash VARCHAR(255) NOT NULL,
		nonce INT NOT NULL,
		transactions JSON
	);`
	_, err := db.Conn.Exec(createBlocksTable)
	return err
}

func (db *MariaDB) SaveBlock(b blockchain.Block) error {
	txJSON, err := json.Marshal(b.Transactions)
	if err != nil {
		return err
	}

	query := `INSERT INTO blocks (block_index, timestamp, prev_hash, hash, nonce, transactions) VALUES (?, ?, ?, ?, ?, ?)`
	_, err = db.Conn.Exec(query, b.Index, b.Timestamp, b.PrevHash, b.Hash, b.Nonce, txJSON)
	return err
}

func (db *MariaDB) LoadBlocks() ([]blockchain.Block, error) {
	rows, err := db.Conn.Query(`SELECT block_index, timestamp, prev_hash, hash, nonce, transactions FROM blocks ORDER BY block_index ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []blockchain.Block
	for rows.Next() {
		var b blockchain.Block
		var txJSON []byte
		err := rows.Scan(&b.Index, &b.Timestamp, &b.PrevHash, &b.Hash, &b.Nonce, &txJSON)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(txJSON, &b.Transactions)
		if err != nil {
			// If JSON unmarshaling fails or it was null, initialize empty slice
			b.Transactions = []blockchain.Transaction{}
		} else if b.Transactions == nil {
			b.Transactions = []blockchain.Transaction{}
		}

		blocks = append(blocks, b)
	}
	return blocks, nil
}

func (db *MariaDB) Close() error {
	if db.Conn != nil {
		return db.Conn.Close()
	}
	return nil
}
