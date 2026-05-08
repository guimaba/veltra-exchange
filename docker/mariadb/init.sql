-- Schema inicial do banco. Executado automaticamente na primeira subida do container.

CREATE DATABASE IF NOT EXISTS blockchain;
USE blockchain;

-- Blocos da cadeia (já existente no projeto base — mantém compatibilidade).
CREATE TABLE IF NOT EXISTS blocks (
  id            BIGINT AUTO_INCREMENT PRIMARY KEY,
  block_index   BIGINT NOT NULL,
  previous_hash VARCHAR(128) NOT NULL,
  hash          VARCHAR(128) NOT NULL,
  nonce         BIGINT NOT NULL,
  timestamp     DATETIME(3) NOT NULL,
  miner_node_id INT NOT NULL,
  UNIQUE KEY uk_block_index (block_index),
  UNIQUE KEY uk_block_hash  (hash)
) ENGINE=InnoDB;

-- Transações dentro de cada bloco.
CREATE TABLE IF NOT EXISTS transactions (
  tx_id      VARCHAR(64) PRIMARY KEY,
  block_id   BIGINT NULL,
  sender     VARCHAR(64) NULL,
  receiver   VARCHAR(64) NOT NULL,
  amount     DECIMAL(18, 2) NOT NULL,
  kind       ENUM('credit', 'transfer') NOT NULL,
  created_at DATETIME(3) NOT NULL,
  KEY idx_tx_sender   (sender),
  KEY idx_tx_receiver (receiver),
  KEY idx_tx_block    (block_id),
  CONSTRAINT fk_tx_block FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE SET NULL
) ENGINE=InnoDB;

-- Idempotência: registra cada mensagem já processada por um consumer.
-- Antes de aplicar efeito, consumer faz INSERT IGNORE; se já existia, pula.
CREATE TABLE IF NOT EXISTS processed_messages (
  tx_id        VARCHAR(64) NOT NULL,
  consumer     VARCHAR(64) NOT NULL,
  processed_at DATETIME(3) NOT NULL,
  PRIMARY KEY (tx_id, consumer)
) ENGINE=InnoDB;

-- Auditoria: persiste todos os eventos consumidos pelo serviço de auditoria.
CREATE TABLE IF NOT EXISTS audit_events (
  id        BIGINT AUTO_INCREMENT PRIMARY KEY,
  schema_id VARCHAR(128) NOT NULL,
  tx_id     VARCHAR(64) NULL,
  payload   JSON NOT NULL,
  recorded_at DATETIME(3) NOT NULL,
  KEY idx_audit_schema (schema_id),
  KEY idx_audit_tx     (tx_id)
) ENGINE=InnoDB;
