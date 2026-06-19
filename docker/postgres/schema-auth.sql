-- Schema auth: usuários, contas de trading, chaves de API, sessões
-- Preparado para evoluir para 1:N de contas (subcontas) sem migração dolorosa

CREATE TABLE IF NOT EXISTS auth.users (
  id BIGSERIAL PRIMARY KEY,
  username VARCHAR(32) NOT NULL UNIQUE,
  email VARCHAR(255) UNIQUE,
  password_hash VARCHAR(255) NOT NULL,
  is_admin BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_auth_users_username ON auth.users(username);
CREATE INDEX idx_auth_users_email ON auth.users(email);

COMMENT ON TABLE auth.users IS 'Usuários da Veltra Exchange';
COMMENT ON COLUMN auth.users.password_hash IS 'Hash argon2id ou bcrypt — nunca plain text';
COMMENT ON COLUMN auth.users.is_admin IS 'Permissão para operações de admin (faucet, reset, etc)';

-- Contas de trading (1:1 com user inicialmente, FK preparada p/ 1:N/subcontas)
CREATE TABLE IF NOT EXISTS auth.accounts (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
  name VARCHAR(64) NOT NULL,  -- "Main", "Bot Trading", etc
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(user_id, name)
);

CREATE INDEX idx_auth_accounts_user_id ON auth.accounts(user_id);

COMMENT ON TABLE auth.accounts IS 'Contas de trading de um usuário; permite 1:N (subcontas) sem migração';
COMMENT ON COLUMN auth.accounts.name IS 'Identificador humano da conta dentro do user';

-- Chaves de API (HMAC para bots/automação) — Fase 3
CREATE TABLE IF NOT EXISTS auth.api_keys (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
  key_id VARCHAR(64) NOT NULL UNIQUE,  -- Público
  secret_hash VARCHAR(255) NOT NULL,    -- Hash do secret (nunca armazenar plain)
  scope VARCHAR(255) NOT NULL DEFAULT 'trading',  -- 'trading', 'read', 'admin'
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  last_used_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMPTZ
);

CREATE INDEX idx_auth_api_keys_key_id ON auth.api_keys(key_id);
CREATE INDEX idx_auth_api_keys_account_id ON auth.api_keys(account_id);

COMMENT ON TABLE auth.api_keys IS 'Chaves de API para acesso via HMAC (Fase 3, plano §5.1)';
COMMENT ON COLUMN auth.api_keys.key_id IS 'Identificador público da chave';
COMMENT ON COLUMN auth.api_keys.secret_hash IS 'Hash do secret — nunca log/retornar';
COMMENT ON COLUMN auth.api_keys.scope IS 'Escopo: trading, read, admin';

-- Sessões de UI web — Fase 3
CREATE TABLE IF NOT EXISTS auth.sessions (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
  session_token VARCHAR(255) NOT NULL UNIQUE,  -- JWT ou opaque token
  ip_address VARCHAR(45),  -- IPv4/IPv6
  user_agent VARCHAR(512),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_active_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_auth_sessions_token ON auth.sessions(session_token);
CREATE INDEX idx_auth_sessions_account_id ON auth.sessions(account_id);
CREATE INDEX idx_auth_sessions_expires_at ON auth.sessions(expires_at);

COMMENT ON TABLE auth.sessions IS 'Sessões autenticadas da UI web; expiram automaticamente';
COMMENT ON COLUMN auth.sessions.session_token IS 'JWT ou opaque token (validar assinatura/TTL)';
