package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ComputeMerkleRoot calcula a raiz Merkle de um conjunto de strings (ex: IDs de postings).
// Algoritmo: hash SHA256 de cada folha, depois combina pares subindo até a raiz.
// Se ímpar, duplica o último nó (comportamento padrão Bitcoin-style).
func ComputeMerkleRoot(leaves []string) string {
	if len(leaves) == 0 {
		return hex.EncodeToString(sha256.New().Sum(nil))
	}

	hashes := make([][]byte, len(leaves))
	for i, l := range leaves {
		h := sha256.Sum256([]byte(l))
		hashes[i] = h[:]
	}

	for len(hashes) > 1 {
		if len(hashes)%2 != 0 {
			hashes = append(hashes, hashes[len(hashes)-1]) // duplica o último
		}
		var next [][]byte
		for i := 0; i < len(hashes); i += 2 {
			// Concatena numa fatia NOVA — append(hashes[i], ...) mutaria o array
			// subjacente de hashes[i] (bug de aliasing) e corromperia a raiz.
			combined := make([]byte, 0, len(hashes[i])+len(hashes[i+1]))
			combined = append(combined, hashes[i]...)
			combined = append(combined, hashes[i+1]...)
			h := sha256.Sum256(combined)
			next = append(next, h[:])
		}
		hashes = next
	}

	return hex.EncodeToString(hashes[0])
}

// SaveMerkleRoot persiste a raiz Merkle de um período no DB.
func (l *Ledger) SaveMerkleRoot(ctx context.Context, start, end time.Time, root string, count int64) error {
	const q = `
		INSERT INTO ledger.merkle_roots (period_start, period_end, root_hash, posting_count)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (period_start, period_end) DO NOTHING
	`
	_, err := l.db.ExecContext(ctx, q, start, end, root, count)
	if err != nil {
		return fmt.Errorf("SaveMerkleRoot: %w", err)
	}
	return nil
}

// BuildAndSaveMerkleRoot coleta todos os postings num período, computa a raiz
// e persiste. Chamado periodicamente (ex: a cada hora) ou por demanda.
func (l *Ledger) BuildAndSaveMerkleRoot(ctx context.Context, start, end time.Time) (string, int64, error) {
	// A folha NÃO inclui created_at (relógio de parede): assim a raiz é
	// reproduzível por replay do log de eventos — os campos restantes são
	// determinísticos dada a sequência de eventos. created_at é usado apenas
	// para delimitar o período auditado, não para o hash.
	const q = `
		SELECT CONCAT(id, ':', ledger_account_id, ':', amount, ':', operation_type, ':', COALESCE(reference_id,''))
		FROM ledger.postings
		WHERE created_at >= $1 AND created_at < $2
		ORDER BY id
	`
	rows, err := l.db.QueryContext(ctx, q, start, end)
	if err != nil {
		return "", 0, fmt.Errorf("BuildAndSaveMerkleRoot query: %w", err)
	}
	defer rows.Close()

	var leaves []string
	for rows.Next() {
		var leaf string
		if err := rows.Scan(&leaf); err != nil {
			return "", 0, fmt.Errorf("BuildAndSaveMerkleRoot scan: %w", err)
		}
		leaves = append(leaves, leaf)
	}
	if err := rows.Err(); err != nil {
		return "", 0, fmt.Errorf("BuildAndSaveMerkleRoot rows: %w", err)
	}

	root := ComputeMerkleRoot(leaves)
	count := int64(len(leaves))

	if err := l.SaveMerkleRoot(ctx, start, end, root, count); err != nil {
		return "", 0, err
	}

	return root, count, nil
}
