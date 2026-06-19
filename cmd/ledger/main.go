// Serviço Ledger da Veltra Exchange.
//
// Consome q.ledger.events (trade.executed, faucet.credit) e realiza o
// settlement de dupla entrada no PostgreSQL. Idempotente via reference_id.
//
// Variáveis de ambiente:
//
//	AMQP_URL (obrigatório)
//	POSTGRES_DSN (obrigatório) — "postgres://veltra:veltra@postgres:5432/veltra?sslmode=disable"
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/ledger"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/messaging"
	"github.com/guimaba/blockchain_sistemasDistribuidos/pkg/pgstore"
)

func main() {
	amqpURL := os.Getenv("AMQP_URL")
	pgDSN := os.Getenv("POSTGRES_DSN")
	if amqpURL == "" || pgDSN == "" {
		log.Fatal("[Ledger] AMQP_URL e POSTGRES_DSN são obrigatórios")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Conecta ao Postgres com retry
	var db *pgstore.DB
	var err error
	for i := 0; i < 30; i++ {
		db, err = pgstore.NewDB(pgDSN)
		if err == nil {
			break
		}
		log.Printf("[Ledger] Aguardando Postgres (%d/30): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("[Ledger] Falha ao conectar ao Postgres: %v", err)
	}
	defer db.Close()
	log.Printf("[Ledger] Conectado ao Postgres")

	l := ledger.NewLedger(db)

	// Garante conta admin (trading_account_id=0) no auth.accounts antes de usar
	if err := ensureAdminAccount(ctx, db); err != nil {
		log.Fatalf("[Ledger] Falha ao garantir conta admin: %v", err)
	}

	// Insere todos os ativos suportados (idempotente) e cria conta _market com liquidez infinita
	if err := seedAssetsAndMarket(ctx, db, l); err != nil {
		log.Printf("[Ledger] Aviso ao seed de ativos/market: %v", err)
	}

	// Conecta ao RabbitMQ
	bootCtx, bootCancel := context.WithTimeout(ctx, 60*time.Second)
	client, err := messaging.NewClient(bootCtx, amqpURL)
	bootCancel()
	if err != nil {
		log.Fatalf("[Ledger] Falha ao conectar ao RabbitMQ: %v", err)
	}
	defer client.Close()

	publisher := messaging.NewPublisher(client)
	defer publisher.Close()

	consumer := messaging.NewConsumer(
		client,
		publisher,
		messaging.ConsumeOptions{
			Queue:    messaging.QueueLedgerEvents,
			Consumer: "ledger",
			Prefetch: 1, // settlement é serializado para garantir consistência
		},
		makeHandler(ctx, l, publisher),
	)
	consumer.Start(ctx)

	log.Printf("[Ledger] Pronto. Consumindo %s", messaging.QueueLedgerEvents)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("[Ledger] Encerrando...")
	consumer.Stop()
}

func makeHandler(ctx context.Context, l *ledger.Ledger, pub *messaging.Publisher) messaging.Handler {
	return func(hctx context.Context, env messaging.Envelope, d amqp.Delivery) error {
		switch d.RoutingKey {
		case "trade.executed":
			return handleTrade(hctx, l, pub, env, d)
		case "faucet.credit":
			return handleFaucet(hctx, l, env, d)
		default:
			// Ignora eventos desconhecidos sem DLQ
			return nil
		}
	}
}

func handleTrade(ctx context.Context, l *ledger.Ledger, pub *messaging.Publisher, env messaging.Envelope, d amqp.Delivery) error {
	var p messaging.TradeExecutedPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return messaging.Permanent("payload trade.executed inválido", err)
	}

	// Resolve qual conta é buyer e qual é seller
	buyer, seller := p.TakerAccount, p.MakerAccount
	if p.TakerSide == "sell" {
		buyer, seller = p.MakerAccount, p.TakerAccount
	}

	// Busca IDs de trading account no auth
	buyerID, err := l.GetOrCreateTradingAccount(ctx, buyer)
	if err != nil {
		return messaging.Transient("erro ao obter trading account buyer", err)
	}
	sellerID, err := l.GetOrCreateTradingAccount(ctx, seller)
	if err != nil {
		return messaging.Transient("erro ao obter trading account seller", err)
	}

	// Extrai par
	baseAsset, quoteAsset, err := parsePair(p.Pair)
	if err != nil {
		return messaging.Permanent("par inválido em trade.executed", err)
	}

	if err := l.SettleTrade(ctx, p.TradeID, buyerID, sellerID, baseAsset, quoteAsset, p.Price, p.Quantity); err != nil {
		return messaging.Transient("erro no settlement do trade", err)
	}

	// Emite ledger.posted com os lançamentos
	entries := []messaging.LedgerEntry{
		{Account: buyer, Asset: baseAsset, Delta: p.Quantity},
		{Account: seller, Asset: baseAsset, Delta: -p.Quantity},
		{Account: seller, Asset: quoteAsset, Delta: notional(p.Price, p.Quantity)},
		{Account: buyer, Asset: quoteAsset, Delta: -notional(p.Price, p.Quantity)},
	}
	_ = publishLedgerPosted(ctx, pub, p.TradeID, entries)

	log.Printf("[Ledger] Trade %s: %s comprou %.8f %s de %s @ %.8f %s",
		p.TradeID, buyer, float64(p.Quantity)/1e8, baseAsset, seller, float64(p.Price)/1e8, quoteAsset)
	return nil
}

func handleFaucet(ctx context.Context, l *ledger.Ledger, env messaging.Envelope, d amqp.Delivery) error {
	var p messaging.FaucetCreditPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return messaging.Permanent("payload faucet.credit inválido", err)
	}

	tradingAcctID, err := l.GetOrCreateTradingAccount(ctx, p.Account)
	if err != nil {
		return messaging.Transient("erro ao obter trading account faucet", err)
	}

	if err := l.SettleFaucet(ctx, tradingAcctID, p.Asset, p.Amount, env.TxID); err != nil {
		return messaging.Transient("erro no settlement do faucet", err)
	}

	log.Printf("[Ledger] Faucet: +%.8f %s → %s", float64(p.Amount)/1e8, p.Asset, p.Account)
	return nil
}

func publishLedgerPosted(ctx context.Context, pub *messaging.Publisher, refID string, entries []messaging.LedgerEntry) error {
	payload := messaging.LedgerPostedPayload{RefTxID: refID, Entries: entries}
	env, err := messaging.NewEnvelope(messaging.SchemaLedgerPosted, refID, payload)
	if err != nil {
		return err
	}
	return pub.Publish(ctx, messaging.ExchangeVeltraEvents, "ledger.posted", env, nil)
}

// parsePair split "VLT/USDT-sim" → ("VLT", "USDT-sim")
func parsePair(pair string) (string, string, error) {
	for i, c := range pair {
		if c == '/' {
			return pair[:i], pair[i+1:], nil
		}
	}
	return "", "", os.ErrInvalid
}

// notional calcula price * quantity / scale (preço do trade em quote asset)
func notional(price, quantity int64) int64 {
	// usa big.Int internamente via money.Notional
	// aqui faz inline para não importar o pacote inteiro
	const scale = 100_000_000
	return price / scale * quantity
}

// ensureAdminAccount garante que existe um registro em auth.accounts com id=0
// (contraconta de emissão usada pelo ledger sink)
func ensureAdminAccount(ctx context.Context, db *pgstore.DB) error {
	const q = `
		INSERT INTO auth.users (id, username, password_hash, is_admin)
		VALUES (0, 'system', 'n/a', true)
		ON CONFLICT (id) DO NOTHING;
		INSERT INTO auth.accounts (id, user_id, name)
		VALUES (0, 0, 'system-sink')
		ON CONFLICT (id) DO NOTHING;
	`
	_, err := db.ExecContext(ctx, q)
	return err
}

// allAssets lista todos os 33 ativos + USDT-sim suportados na exchange simulada.
var allAssets = []string{
	"USDT-sim",
	"VLT", "BTC", "ETH", "BNB", "SOL", "XRP", "ADA", "DOGE", "DOT", "AVAX",
	"POL", "LINK", "UNI", "LTC", "FIL", "ALGO", "XLM", "NEAR", "ICP", "APT",
	"ARB", "OP", "INJ", "SEI", "SUI", "TIA", "PEPE", "SHIB", "WIF", "JUP",
	"BONK", "TON", "TRX",
}

// seedAssetsAndMarket registra todos os ativos no ledger e cria a conta _market
// com liquidez "infinita" (1e16 scaled = 1e8 unidades reais de cada ativo).
func seedAssetsAndMarket(ctx context.Context, db *pgstore.DB, l *ledger.Ledger) error {
	// 1. Insere ativos que ainda não existem
	for _, asset := range allAssets {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO ledger.assets (code, scale) VALUES ($1, 100000000) ON CONFLICT (code) DO NOTHING`,
			asset,
		); err != nil {
			return err
		}
	}

	// 2. Cria usuário _market (fornecedor de liquidez simulada)
	var marketUserID int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO auth.users (username, password_hash, is_admin)
		 VALUES ('_market', 'n/a', true)
		 ON CONFLICT (username) DO UPDATE SET username = EXCLUDED.username
		 RETURNING id`,
	).Scan(&marketUserID)
	if err != nil {
		return err
	}
	var marketAcctID int64
	err = db.QueryRowContext(ctx,
		`INSERT INTO auth.accounts (user_id, name)
		 VALUES ($1, '_market')
		 ON CONFLICT (user_id, name) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id`,
		marketUserID,
	).Scan(&marketAcctID)
	if err != nil {
		return err
	}

	// 3. Seed das contas ledger do _market com saldo inicial grande (idempotente via ON CONFLICT)
	const marketBalance int64 = 1_000_000_000_000_000_000 // 1e18 scaled
	for _, asset := range allAssets {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO ledger.accounts (trading_account_id, asset, balance)
			VALUES ($1, $2, $3)
			ON CONFLICT (trading_account_id, asset) DO NOTHING`,
			marketAcctID, asset, marketBalance,
		); err != nil {
			return err
		}
	}

	// 4. Seed das contas sink (trading_account_id=0) para os novos ativos
	for _, asset := range allAssets {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO ledger.accounts (trading_account_id, asset, balance)
			VALUES (0, $1, -922337203685477580)
			ON CONFLICT (trading_account_id, asset) DO NOTHING`,
			asset,
		); err != nil {
			return err
		}
	}

	log.Printf("[Ledger] Ativos e conta _market (id=%d) prontos", marketAcctID)
	return nil
}
