package ledger

import (
	"context"
	"fmt"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/money"
)

// SettleTrade movimenta saldos após trade.executed.
// buyer/seller são nomes de conta (string), tradingAcctIDs são os IDs do schema auth.
// Invariante: soma de postings por ativo = 0 (balanceado).
func (l *Ledger) SettleTrade(
	ctx context.Context,
	tradeID string,
	buyerTradingAcctID int64,
	sellerTradingAcctID int64,
	baseAsset string,
	quoteAsset string,
	price int64, // escalado (money.Scale)
	quantity int64, // escalado
) error {
	sellerBase, err := l.GetOrCreateAccount(ctx, sellerTradingAcctID, baseAsset)
	if err != nil {
		return fmt.Errorf("settlement seller base account: %w", err)
	}
	sellerQuote, err := l.GetOrCreateAccount(ctx, sellerTradingAcctID, quoteAsset)
	if err != nil {
		return fmt.Errorf("settlement seller quote account: %w", err)
	}
	buyerBase, err := l.GetOrCreateAccount(ctx, buyerTradingAcctID, baseAsset)
	if err != nil {
		return fmt.Errorf("settlement buyer base account: %w", err)
	}
	buyerQuote, err := l.GetOrCreateAccount(ctx, buyerTradingAcctID, quoteAsset)
	if err != nil {
		return fmt.Errorf("settlement buyer quote account: %w", err)
	}

	notional, err := money.Notional(money.Amount(price), money.Amount(quantity))
	if err != nil {
		return fmt.Errorf("settlement notional overflow: %w", err)
	}
	desc := fmt.Sprintf("trade settlement %s", tradeID)

	// As pernas base e quote são liquidadas numa ÚNICA transação: ou ambas
	// entram, ou nenhuma — nunca um meio-trade (plano §4.3: "move saldo de forma
	// atômica"). Idempotente: se a perna base já foi aplicada (redelivery), a
	// transação inteira é abortada sem reaplicar a quote.
	tx, err := l.db.Conn().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("settlement begin tx: %w", err)
	}
	defer tx.Rollback()

	// Base: buyer +quantity, seller -quantity
	applied, err := l.postEntryTx(ctx, tx, buyerBase.ID, sellerBase.ID, quantity, "trade", tradeID, desc)
	if err != nil {
		return fmt.Errorf("settlement base posting: %w", err)
	}
	if !applied {
		// Trade já liquidado anteriormente → idempotente, nada a fazer.
		return nil
	}

	// Quote: seller +notional, buyer -notional
	if _, err := l.postEntryTx(ctx, tx, sellerQuote.ID, buyerQuote.ID, int64(notional), "trade", tradeID+"-q", desc); err != nil {
		return fmt.Errorf("settlement quote posting: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("settlement commit: %w", err)
	}
	return nil
}

// SettleFaucet credita um asset a uma conta (operação admin).
// Usa uma contraconta de emissão (sink) com ID fixo definido via seed.
func (l *Ledger) SettleFaucet(ctx context.Context, tradingAcctID int64, asset string, amount int64, txID string) error {
	acct, err := l.GetOrCreateAccount(ctx, tradingAcctID, asset)
	if err != nil {
		return fmt.Errorf("settlement faucet account: %w", err)
	}

	// Conta de emissão do asset (seed no bootstrap do ledger)
	sinkID, err := l.getSinkAccountID(ctx, asset)
	if err != nil {
		return fmt.Errorf("settlement faucet sink: %w", err)
	}

	desc := fmt.Sprintf("faucet credit %s", asset)
	if err := l.PostEntry(ctx, acct.ID, sinkID, amount, "faucet", txID, desc); err != nil {
		return fmt.Errorf("settlement faucet posting: %w", err)
	}

	return nil
}

// getSinkAccountID retorna o ID da conta de emissão do asset.
// A conta sink é criada no bootstrap e tem trading_account_id = 0 (admin).
func (l *Ledger) getSinkAccountID(ctx context.Context, asset string) (int64, error) {
	const q = `SELECT id FROM ledger.accounts WHERE trading_account_id = 0 AND asset = $1`
	var id int64
	if err := l.db.QueryRowContext(ctx, q, asset).Scan(&id); err != nil {
		return 0, fmt.Errorf("getSinkAccountID %s: %w", asset, err)
	}
	return id, nil
}
