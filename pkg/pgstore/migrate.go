package pgstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Migrate aplica os arquivos .sql de schemaDir (em ordem alfabética) UMA vez.
//
// No Docker Compose o Postgres roda os scripts de docker-entrypoint-initdb.d
// automaticamente; o RDS NÃO faz isso, então o serviço aplica o schema no
// startup. Idempotente: se a tabela auth.users já existe, não faz nada (cobre
// o caso do Compose, onde o initdb já criou o schema antes do serviço conectar).
func (d *DB) Migrate(ctx context.Context, schemaDir string) error {
	if schemaDir == "" {
		return nil
	}

	// Já migrado? (to_regclass retorna NULL se a tabela não existe)
	var existing *string
	if err := d.conn.QueryRowContext(ctx, "SELECT to_regclass('auth.users')::text").Scan(&existing); err != nil {
		return fmt.Errorf("migrate check: %w", err)
	}
	if existing != nil {
		return nil // schema já presente
	}

	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil // diretório ausente — nada a migrar
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files) // init.sql < schema-auth.sql < schema-ledger.sql

	for _, f := range files {
		sqlBytes, err := os.ReadFile(filepath.Join(schemaDir, f))
		if err != nil {
			return fmt.Errorf("migrate read %s: %w", f, err)
		}
		if _, err := d.conn.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("migrate exec %s: %w", f, err)
		}
	}
	return nil
}
