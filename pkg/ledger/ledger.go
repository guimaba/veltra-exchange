package ledger

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/pgstore"
)

type Ledger struct {
	db *pgstore.DB
}

type Account struct {
	ID                int64
	TradingAccountID  int64
	Asset             string
	Balance           int64 // int64 escalado (money.Scale)
	Reserved          int64 // holds
	Available         int64 // balance - reserved (GENERATED)
}

type Posting struct {
	ID            int64
	AccountID     int64
	Amount        int64
	OperationType string
	ReferenceID   string
	Description   string
	CreatedAt     string
}

type Hold struct {
	ID           int64
	AccountID    int64
	OrderID      string
	Amount       int64
	Reason       string
	CreatedAt    string
	ReleasedAt   *string
}

// NewLedger cria uma instância do motor de ledger
func NewLedger(db *pgstore.DB) *Ledger {
	return &Ledger{db: db}
}

// GetOrCreateAccount obtém a conta de ledger para um asset, ou cria se não existe
func (l *Ledger) GetOrCreateAccount(ctx context.Context, tradingAcctID int64, asset string) (*Account, error) {
	const q = `
		INSERT INTO ledger.accounts (trading_account_id, asset)
		VALUES ($1, $2)
		ON CONFLICT (trading_account_id, asset) DO NOTHING
		RETURNING id, trading_account_id, asset, balance, reserved, available
	`
	row := l.db.QueryRowContext(ctx, q, tradingAcctID, asset)
	var acct Account
	err := row.Scan(&acct.ID, &acct.TradingAccountID, &acct.Asset, &acct.Balance, &acct.Reserved, &acct.Available)
	if err == sql.ErrNoRows {
		// Já existe, buscar
		const qGet = `SELECT id, trading_account_id, asset, balance, reserved, available FROM ledger.accounts WHERE trading_account_id=$1 AND asset=$2`
		row := l.db.QueryRowContext(ctx, qGet, tradingAcctID, asset)
		err := row.Scan(&acct.ID, &acct.TradingAccountID, &acct.Asset, &acct.Balance, &acct.Reserved, &acct.Available)
		if err != nil {
			return nil, fmt.Errorf("ledger.GetOrCreateAccount scan: %w", err)
		}
		return &acct, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger.GetOrCreateAccount insert: %w", err)
	}
	return &acct, nil
}

// GetAccount retorna a conta de ledger
func (l *Ledger) GetAccount(ctx context.Context, tradingAcctID int64, asset string) (*Account, error) {
	const q = `SELECT id, trading_account_id, asset, balance, reserved, available FROM ledger.accounts WHERE trading_account_id=$1 AND asset=$2`
	row := l.db.QueryRowContext(ctx, q, tradingAcctID, asset)
	var acct Account
	err := row.Scan(&acct.ID, &acct.TradingAccountID, &acct.Asset, &acct.Balance, &acct.Reserved, &acct.Available)
	if err != nil {
		return nil, fmt.Errorf("ledger.GetAccount: %w", err)
	}
	return &acct, nil
}

// postEntryTx aplica um par débito/crédito balanceado DENTRO de uma transação
// já aberta. É IDEMPOTENTE: o INSERT do débito usa
// ON CONFLICT (ledger_account_id, reference_id) DO NOTHING; se nada foi inserido
// (reference já aplicada para esta conta), retorna applied=false e não toca em
// nenhum saldo — o redelivery do RabbitMQ não duplica lançamentos.
// debitAcctID += amount, creditAcctID -= amount.
func (l *Ledger) postEntryTx(ctx context.Context, tx *sql.Tx, debitAcctID, creditAcctID, amount int64, opType, referenceID, desc string) (applied bool, err error) {
	const qIns = `INSERT INTO ledger.postings (ledger_account_id, amount, operation_type, reference_id, description)
	              VALUES ($1, $2, $3, $4, $5)
	              ON CONFLICT (ledger_account_id, reference_id) DO NOTHING`

	res, err := tx.ExecContext(ctx, qIns, debitAcctID, amount, opType, referenceID, desc)
	if err != nil {
		return false, fmt.Errorf("ledger.postEntryTx debit: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Já aplicado (mesma conta+referência) → idempotente, nada a fazer.
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, qIns, creditAcctID, -amount, opType, referenceID, desc); err != nil {
		return false, fmt.Errorf("ledger.postEntryTx credit: %w", err)
	}

	const qUpdateDebit = `UPDATE ledger.accounts SET balance = balance + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	if _, err := tx.ExecContext(ctx, qUpdateDebit, amount, debitAcctID); err != nil {
		return false, fmt.Errorf("ledger.postEntryTx update debit: %w", err)
	}

	const qUpdateCredit = `UPDATE ledger.accounts SET balance = balance - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	if _, err := tx.ExecContext(ctx, qUpdateCredit, amount, creditAcctID); err != nil {
		return false, fmt.Errorf("ledger.postEntryTx update credit: %w", err)
	}
	return true, nil
}

// PostEntry cria um par débito/crédito balanceado em duas contas, em sua própria
// transação. Idempotente via reference_id (ver postEntryTx).
func (l *Ledger) PostEntry(ctx context.Context, debitAcctID, creditAcctID, amount int64, opType, referenceID, desc string) error {
	tx, err := l.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger.PostEntry BeginTx: %w", err)
	}
	defer tx.Rollback()

	if _, err := l.postEntryTx(ctx, tx, debitAcctID, creditAcctID, amount, opType, referenceID, desc); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ledger.PostEntry commit: %w", err)
	}
	return nil
}

// HoldAmount bloqueia saldo por ordem aberta
func (l *Ledger) HoldAmount(ctx context.Context, acctID int64, orderID string, amount int64, reason string) error {
	const q = `INSERT INTO ledger.holds (ledger_account_id, order_id, amount, reason) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`
	if _, err := l.db.ExecContext(ctx, q, acctID, orderID, amount, reason); err != nil {
		return fmt.Errorf("ledger.HoldAmount: %w", err)
	}
	// Atualizar reserved
	const qUpdate = `UPDATE ledger.accounts SET reserved = reserved + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	if _, err := l.db.ExecContext(ctx, qUpdate, amount, acctID); err != nil {
		return fmt.Errorf("ledger.HoldAmount update: %w", err)
	}
	return nil
}

// ReleaseHold libera um hold (em cancel/reject/fill)
func (l *Ledger) ReleaseHold(ctx context.Context, acctID int64, orderID string) (int64, error) {
	// Busca o hold
	const qGet = `SELECT amount FROM ledger.holds WHERE ledger_account_id=$1 AND order_id=$2 AND released_at IS NULL`
	var amount int64
	err := l.db.QueryRowContext(ctx, qGet, acctID, orderID).Scan(&amount)
	if err == sql.ErrNoRows {
		return 0, nil // hold não existe, ok
	}
	if err != nil {
		return 0, fmt.Errorf("ledger.ReleaseHold query: %w", err)
	}

	// Marca hold como liberado
	const qRelease = `UPDATE ledger.holds SET released_at = CURRENT_TIMESTAMP WHERE ledger_account_id=$1 AND order_id=$2`
	if _, err := l.db.ExecContext(ctx, qRelease, acctID, orderID); err != nil {
		return 0, fmt.Errorf("ledger.ReleaseHold update: %w", err)
	}

	// Atualiza reserved
	const qUpdate = `UPDATE ledger.accounts SET reserved = reserved - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	if _, err := l.db.ExecContext(ctx, qUpdate, amount, acctID); err != nil {
		return 0, fmt.Errorf("ledger.ReleaseHold update reserved: %w", err)
	}

	return amount, nil
}

// ReserveIfAvailable reserva `amount` do ativo na conta de forma ATÔMICA e
// condicional: o saldo só é reservado se houver disponível suficiente, decidido
// na MESMA instrução UPDATE (WHERE balance-reserved >= amount). Isso elimina a
// janela de corrida (TOCTOU) entre "validar saldo" e "reservar" — duas ordens
// concorrentes não conseguem reservar o mesmo saldo. Idempotente por orderID
// (um redelivery/retry não reserva em dobro). Retorna ok=false se insuficiente.
func (l *Ledger) ReserveIfAvailable(ctx context.Context, tradingAcctID int64, asset, orderID string, amount int64, reason string) (bool, error) {
	if amount <= 0 {
		return true, nil // nada a reservar
	}
	tx, err := l.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("ReserveIfAvailable begin: %w", err)
	}
	defer tx.Rollback()

	// Garante a conta-ativo e obtém seu id.
	var acctID int64
	const qAcct = `INSERT INTO ledger.accounts (trading_account_id, asset) VALUES ($1, $2)
	               ON CONFLICT (trading_account_id, asset) DO UPDATE SET trading_account_id = EXCLUDED.trading_account_id
	               RETURNING id`
	if err := tx.QueryRowContext(ctx, qAcct, tradingAcctID, asset).Scan(&acctID); err != nil {
		return false, fmt.Errorf("ReserveIfAvailable acct: %w", err)
	}

	// Registro de hold idempotente: se já existe hold para este order, considera
	// reservado (não soma de novo).
	const qHold = `INSERT INTO ledger.holds (ledger_account_id, order_id, amount, reason)
	               VALUES ($1, $2, $3, $4) ON CONFLICT (ledger_account_id, order_id) DO NOTHING`
	resHold, err := tx.ExecContext(ctx, qHold, acctID, orderID, amount, reason)
	if err != nil {
		return false, fmt.Errorf("ReserveIfAvailable hold: %w", err)
	}
	if n, _ := resHold.RowsAffected(); n == 0 {
		return true, tx.Commit() // já reservado anteriormente → idempotente
	}

	// Reserva condicional atômica: só aplica se houver disponível.
	const qReserve = `UPDATE ledger.accounts SET reserved = reserved + $1, updated_at = CURRENT_TIMESTAMP
	                  WHERE id = $2 AND (balance - reserved) >= $1`
	resRes, err := tx.ExecContext(ctx, qReserve, amount, acctID)
	if err != nil {
		return false, fmt.Errorf("ReserveIfAvailable reserve: %w", err)
	}
	if n, _ := resRes.RowsAffected(); n == 0 {
		// Saldo insuficiente → rollback (desfaz o INSERT do hold) e recusa.
		return false, nil
	}
	return true, tx.Commit()
}

// ReleaseReserveByOrder libera os holds ativos de um orderID (devolve o saldo
// reservado ao disponível). Chamado quando a ordem atinge estado terminal
// (filled total / canceled / rejected). Liberar o hold cheio no fechamento é
// correto mesmo com price improvement: o débito real já moveu `balance` no
// settlement; aqui só desfazemos a reserva original.
func (l *Ledger) ReleaseReserveByOrder(ctx context.Context, orderID string) error {
	if orderID == "" {
		return nil
	}
	tx, err := l.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ReleaseReserveByOrder begin: %w", err)
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT ledger_account_id, amount FROM ledger.holds WHERE order_id = $1 AND released_at IS NULL`, orderID)
	if err != nil {
		return fmt.Errorf("ReleaseReserveByOrder query: %w", err)
	}
	type held struct{ acctID, amount int64 }
	var holds []held
	for rows.Next() {
		var h held
		if err := rows.Scan(&h.acctID, &h.amount); err != nil {
			rows.Close()
			return fmt.Errorf("ReleaseReserveByOrder scan: %w", err)
		}
		holds = append(holds, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ReleaseReserveByOrder rows: %w", err)
	}

	for _, h := range holds {
		if _, err := tx.ExecContext(ctx,
			`UPDATE ledger.holds SET released_at = CURRENT_TIMESTAMP WHERE ledger_account_id = $1 AND order_id = $2 AND released_at IS NULL`,
			h.acctID, orderID); err != nil {
			return fmt.Errorf("ReleaseReserveByOrder release: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE ledger.accounts SET reserved = reserved - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
			h.amount, h.acctID); err != nil {
			return fmt.Errorf("ReleaseReserveByOrder update reserved: %w", err)
		}
	}
	return tx.Commit()
}

// GetBalance retorna o saldo total (soma dos postings) de uma conta
func (l *Ledger) GetBalance(ctx context.Context, acctID int64) (int64, error) {
	const q = `SELECT balance FROM ledger.accounts WHERE id = $1`
	var balance int64
	err := l.db.QueryRowContext(ctx, q, acctID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("ledger.GetBalance: %w", err)
	}
	return balance, nil
}

// GetAvailable retorna saldo disponível (balance - reserved)
func (l *Ledger) GetAvailable(ctx context.Context, acctID int64) (int64, error) {
	const q = `SELECT available FROM ledger.accounts WHERE id = $1`
	var available int64
	err := l.db.QueryRowContext(ctx, q, acctID).Scan(&available)
	if err != nil {
		return 0, fmt.Errorf("ledger.GetAvailable: %w", err)
	}
	return available, nil
}

// ValidateSufficient valida se há saldo disponível para uma operação
func (l *Ledger) ValidateSufficient(ctx context.Context, acctID int64, required int64) (bool, error) {
	available, err := l.GetAvailable(ctx, acctID)
	if err != nil {
		return false, err
	}
	return available >= required, nil
}

// GetOrCreateTradingAccount retorna o ID de auth.accounts pelo nome da conta,
// criando user + account se ainda não existirem (auto-provisionamento para a
// fase atual, pré-autenticação; será substituído por lookup puro na Fase 3).
func (l *Ledger) GetOrCreateTradingAccount(ctx context.Context, name string) (int64, error) {
	const qGet = `SELECT id FROM auth.accounts WHERE name = $1 LIMIT 1`
	var id int64
	err := l.db.QueryRowContext(ctx, qGet, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("GetOrCreateTradingAccount query: %w", err)
	}

	// Cria user + account em transação
	tx, err := l.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("GetOrCreateTradingAccount begin: %w", err)
	}
	defer tx.Rollback()

	var userID int64
	const qInsertUser = `INSERT INTO auth.users (username, password_hash) VALUES ($1, 'auto') ON CONFLICT (username) DO UPDATE SET username=EXCLUDED.username RETURNING id`
	if err := tx.QueryRowContext(ctx, qInsertUser, name).Scan(&userID); err != nil {
		return 0, fmt.Errorf("GetOrCreateTradingAccount insert user: %w", err)
	}

	const qInsertAcct = `INSERT INTO auth.accounts (user_id, name) VALUES ($1, $2) ON CONFLICT (user_id, name) DO UPDATE SET name=EXCLUDED.name RETURNING id`
	if err := tx.QueryRowContext(ctx, qInsertAcct, userID, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("GetOrCreateTradingAccount insert account: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("GetOrCreateTradingAccount commit: %w", err)
	}
	return id, nil
}
