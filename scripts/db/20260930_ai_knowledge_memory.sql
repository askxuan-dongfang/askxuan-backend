-- Lightweight references: no existing conversations are silently imported.
CREATE TABLE IF NOT EXISTS ai_reference_scope (scope_key VARCHAR(160) PRIMARY KEY) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS ai_reference_entry (
 id VARCHAR(36) PRIMARY KEY, owner VARCHAR(80) NOT NULL, kind VARCHAR(24) NOT NULL,
 title VARCHAR(480) NOT NULL, body TEXT NOT NULL, source VARCHAR(1000) NOT NULL, locator VARCHAR(500) NOT NULL,
 enabled BOOLEAN NOT NULL DEFAULT FALSE, revision BIGINT NOT NULL DEFAULT 1, expires BIGINT NOT NULL DEFAULT 0,
 vector_json MEDIUMTEXT NOT NULL, embedding_model VARCHAR(255) NOT NULL DEFAULT '',
 INDEX idx_reference_owner (kind,owner)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS ai_aux_usage_log (
 id VARCHAR(36) PRIMARY KEY, user_id VARCHAR(128) NOT NULL, purpose VARCHAR(64) NOT NULL,
 provider VARCHAR(64) NOT NULL, model VARCHAR(128) NOT NULL, prompt_tokens INT NOT NULL,
 completion_tokens INT NOT NULL,cost_micros BIGINT NOT NULL,status VARCHAR(32) NOT NULL,
 latency_ms INT NOT NULL,create_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY idx_user_created(user_id,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
