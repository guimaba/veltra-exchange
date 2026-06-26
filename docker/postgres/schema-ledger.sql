-- Schema ledger: razão de dupla entrada, saldos, holds
-- Modelo: débito/crédito balanceado (invariante: soma = 0 por ativo)

-- Ativos conhecidos (seed básico — pode vir de enum Go também)
CREATE TABLE IF NOT EXISTS ledger.assets (
  code VARCHAR(32) PRIMARY KEY,
  scale BIGINT NOT NULL,  -- casas decimais: VLT/USDT-sim = 1e8
  is_active BOOLEAN NOT NULL DEFAULT TRUE
);

INSERT INTO ledger.assets (code, scale) VALUES ('VLT', 100000000), ('USDT-sim', 100000000)
  ON CONFLICT (code) DO NOTHING;

COMMENT ON TABLE ledger.assets IS 'Ativos suportados; scale = precisão de casas decimais';

-- Contas contábeis por asset (não confundir com accounts de trading em auth.accounts)
-- trading_account_id = 0 é reservado para contas sink de emissão (admin/sistema)
-- Cada auth.account tem uma ledger.account POR ATIVO
CREATE TABLE IF NOT EXISTS ledger.accounts (
  id BIGSERIAL PRIMARY KEY,
  trading_account_id BIGINT,  -- NULL ou 0 = conta sink de emissão
  asset VARCHAR(32) NOT NULL REFERENCES ledger.assets(code) ON DELETE RESTRICT,
  balance BIGINT NOT NULL DEFAULT 0,  -- soma de postings; int64 escalado
  reserved BIGINT NOT NULL DEFAULT 0,  -- somas de holds (não disponível)
  available BIGINT GENERATED ALWAYS AS (balance - reserved) STORED,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(trading_account_id, asset)
);

-- Contas sink (emissão): trading_account_id = 0 por convenção Go / NULL no DB
INSERT INTO ledger.accounts (id, trading_account_id, asset, balance) VALUES
  (1, 0, 'VLT',      -922337203685477580),  -- saldo inicial "infinito" negativo (contra-conta emissão)
  (2, 0, 'USDT-sim', -922337203685477580)
ON CONFLICT DO NOTHING;
ALTER SEQUENCE ledger.accounts_id_seq RESTART WITH 100;

CREATE INDEX idx_ledger_accounts_trading_id ON ledger.accounts(trading_account_id);
CREATE INDEX idx_ledger_accounts_asset ON ledger.accounts(asset);

COMMENT ON TABLE ledger.accounts IS 'Saldo contábil por account/asset; disponível = balance - reserved';
COMMENT ON COLUMN ledger.accounts.balance IS 'Saldo total (soma de postings) em unidade escalada';
COMMENT ON COLUMN ledger.accounts.reserved IS 'Saldo reservado por ordens abertas (holds)';

-- Lançamentos (débito/crédito): dupla entrada
-- Cada operação (trade, faucet, etc) cria SEMPRE dois postings (um débito, um crédito)
CREATE TABLE IF NOT EXISTS ledger.postings (
  id BIGSERIAL PRIMARY KEY,
  ledger_account_id BIGINT NOT NULL REFERENCES ledger.accounts(id) ON DELETE CASCADE,
  amount BIGINT NOT NULL,  -- sempre positivo; débito=+, crédito=-(calculado na UI)
  operation_type VARCHAR(32) NOT NULL,  -- 'trade', 'faucet', 'settlement', etc
  reference_id VARCHAR(255),  -- tradeID, orderID, txID para idempotência
  description TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_ledger_postings_account ON ledger.postings(ledger_account_id);
-- Idempotência REAL: cada (conta, referência) só pode ser lançada uma vez.
-- Habilita ON CONFLICT DO NOTHING em ledger.postEntryTx → redelivery do RabbitMQ
-- (entrega at-least-once) nunca duplica saldo. O débito e o crédito de um mesmo
-- par usam a mesma reference_id em CONTAS diferentes, então não colidem entre si.
CREATE UNIQUE INDEX idx_ledger_postings_account_ref ON ledger.postings(ledger_account_id, reference_id);
CREATE INDEX idx_ledger_postings_created ON ledger.postings(created_at);

COMMENT ON TABLE ledger.postings IS 'Lançamentos contábeis (sempre com contrapartida); base imutável';
COMMENT ON COLUMN ledger.postings.amount IS 'Valor sempre positivo; semântica débito/crédito no design';
COMMENT ON COLUMN ledger.postings.reference_id IS 'Identificador único para idempotência (tradeID, orderID, etc)';

-- Holds (reservas): saldo bloqueado por ordem aberta
-- Quando a ordem executa/cancela, o hold é liberado
CREATE TABLE IF NOT EXISTS ledger.holds (
  id BIGSERIAL PRIMARY KEY,
  ledger_account_id BIGINT NOT NULL REFERENCES ledger.accounts(id) ON DELETE CASCADE,
  order_id VARCHAR(36) NOT NULL,  -- UUID da ordem no matching
  amount BIGINT NOT NULL,  -- reservado para esta ordem
  reason VARCHAR(64) NOT NULL,  -- 'sell_quantity', 'buy_notional', etc
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  released_at TIMESTAMPTZ,
  UNIQUE(ledger_account_id, order_id)
);

CREATE INDEX idx_ledger_holds_account ON ledger.holds(ledger_account_id);
CREATE INDEX idx_ledger_holds_order_id ON ledger.holds(order_id);
CREATE INDEX idx_ledger_holds_released ON ledger.holds(released_at) WHERE released_at IS NULL;

COMMENT ON TABLE ledger.holds IS 'Reservas de saldo por ordem aberta; liberadas em fill/cancel/reject';
COMMENT ON COLUMN ledger.holds.reason IS 'Semântica: sell_quantity (VLT reservado), buy_notional (USDT reservado)';

-- Merkle roots por período (auditoria — Fase 2, T2.5)
CREATE TABLE IF NOT EXISTS ledger.merkle_roots (
  id BIGSERIAL PRIMARY KEY,
  period_start TIMESTAMPTZ NOT NULL,
  period_end TIMESTAMPTZ NOT NULL,
  root_hash VARCHAR(128) NOT NULL,  -- SHA256 hex (64 bytes)
  posting_count BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(period_start, period_end)
);

CREATE INDEX idx_ledger_merkle_period ON ledger.merkle_roots(period_start, period_end);

COMMENT ON TABLE ledger.merkle_roots IS 'Merkle root de auditoria por período (reuso pkg/blockchain)';
COMMENT ON COLUMN ledger.merkle_roots.root_hash IS 'Raiz SHA256 dos postings do período (prova sem expor ledger)';
