-- Inicialização dos schemas da Veltra Exchange
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS ledger;

COMMENT ON SCHEMA auth IS 'Autenticação, usuários, contas e sessões';
COMMENT ON SCHEMA ledger IS 'Razão de dupla entrada, saldos, holds, movimentações';

-- Sequência reservada para IDs de conta admin (começa em 1, 0 reservado para sink)
-- A conta sink é virtual (trading_account_id=0) e representa emissão/destruição de ativos
