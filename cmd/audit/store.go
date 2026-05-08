package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const consumerName = "audit"

// Store encapsula a persistencia do servico de auditoria. Duas tabelas
// envolvidas (criadas pelo docker/mariadb/init.sql):
//   - processed_messages: garante idempotencia (PK composta tx_id+consumer)
//   - audit_events: registra cada evento recebido
type Store struct {
	db *sql.DB
}

// NewStore conecta no MariaDB com retry simples (espera o container subir).
func NewStore(ctx context.Context, dsn string) (*Store, error) {
	var lastErr error
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			if err = db.PingContext(ctx); err == nil {
				db.SetMaxOpenConns(5)
				db.SetMaxIdleConns(2)
				db.SetConnMaxLifetime(5 * time.Minute)
				return &Store{db: db}, nil
			}
			db.Close()
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, fmt.Errorf("nao conseguiu conectar ao MariaDB: %w", lastErr)
}

func (s *Store) Close() error { return s.db.Close() }

// errAlreadyProcessed sinaliza que essa mensagem ja foi gravada por este consumer.
var errAlreadyProcessed = errors.New("mensagem ja processada")

// markProcessed insere o tx_id na tabela de idempotencia. Se ja existia,
// retorna errAlreadyProcessed.
func (s *Store) markProcessed(ctx context.Context, txID string) error {
	if txID == "" {
		// Mensagens sem tx_id nao podem ser idempotentes; permite duplicar.
		return nil
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT IGNORE INTO processed_messages (tx_id, consumer, processed_at) VALUES (?, ?, ?)`,
		txID, consumerName, time.Now().UTC(),
	)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errAlreadyProcessed
	}
	return nil
}

// SaveEvent registra um evento na tabela audit_events. Idempotente: se a
// mensagem (tx_id, consumer) ja existe em processed_messages, pula a
// insercao do audit (retorna nil).
func (s *Store) SaveEvent(ctx context.Context, schema, txID string, payload []byte) error {
	if err := s.markProcessed(ctx, txID); err != nil {
		if errors.Is(err, errAlreadyProcessed) {
			return nil
		}
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_events (schema_id, tx_id, payload, recorded_at) VALUES (?, ?, ?, ?)`,
		schema, nullable(txID), payload, time.Now().UTC(),
	)
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
