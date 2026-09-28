-- Apply after 20260928_ai_agent_operations.sql, to the AI service database.
CREATE TABLE IF NOT EXISTS ai_agent_checkpoint (
  id VARCHAR(36) PRIMARY KEY,
  ciphertext LONGTEXT NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  owner_token VARCHAR(36) NOT NULL DEFAULT '',
  lease_until DATETIME(3) NULL,
  INDEX idx_agent_checkpoint_expiry(expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ai_agent_evaluation (
  id VARCHAR(36) PRIMARY KEY,
  actor VARCHAR(64) NOT NULL,
  revision BIGINT NOT NULL,
  provider_revision BIGINT NOT NULL,
  suite_hash CHAR(64) NOT NULL,
  status VARCHAR(24) NOT NULL,
  payload LONGTEXT NOT NULL,
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  INDEX idx_agent_eval_gate(revision,provider_revision,suite_hash,status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ai_agent_rollout (
  id TINYINT PRIMARY KEY,
  version_id BIGINT NOT NULL DEFAULT 0,
  stable_version BIGINT NOT NULL DEFAULT 0,
  percentage INT NOT NULL DEFAULT 0,
  revision BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT IGNORE INTO ai_agent_rollout(id) VALUES(1);
