package pgstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	conn *sql.DB
}

// NewDB abre conexão com Postgres usando DSN no formato:
// postgres://user:password@host:port/database?sslmode=disable
func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close encerra a conexão
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn retorna a conexão bruta (para transações, etc)
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// ExecContext executa um comando (INSERT/UPDATE/DELETE)
func (d *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.conn.ExecContext(ctx, query, args...)
}

// QueryRowContext executa uma query e retorna uma linha
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return d.conn.QueryRowContext(ctx, query, args...)
}

// QueryContext executa uma query e retorna múltiplas linhas
func (d *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.conn.QueryContext(ctx, query, args...)
}
