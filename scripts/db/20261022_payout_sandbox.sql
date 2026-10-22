-- Independent payout simulator. No real account, recipient, ledger or withdrawal data.
-- Apply with a DDL operator to askxuan_finance. Runtime needs only SELECT/INSERT/UPDATE
-- on payout_sandbox and SELECT/INSERT on payout_sandbox_event for this feature.
-- No DELETE, cross-schema, bank/provider or message-queue permission is required.
-- Example minimum grants (replace runtime principal; execute separately):
-- GRANT SELECT, INSERT, UPDATE ON askxuan_finance.payout_sandbox TO '<runtime_user>'@'<runtime_host>';
-- GRANT SELECT, INSERT ON askxuan_finance.payout_sandbox_event TO '<runtime_user>'@'<runtime_host>';
USE askxuan_finance;

CREATE TABLE IF NOT EXISTS payout_sandbox (
  id VARCHAR(44) NOT NULL PRIMARY KEY,
  create_request_id CHAR(36) NOT NULL,
  amount_cents BIGINT NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'pending_review',
  version BIGINT NOT NULL DEFAULT 1,
  attempt INT NOT NULL DEFAULT 0,
  created_by VARCHAR(32) NOT NULL,
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  update_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  UNIQUE KEY payout_sandbox_request (create_request_id),
  KEY payout_sandbox_status (status,create_time,id),
  CHECK (amount_cents BETWEEN 1 AND 1000000),
  CHECK (version >= 1 AND attempt >= 0),
  CHECK (status IN ('pending_review','approved','rejected','success','failed','unknown')),
  CHECK (LEFT(id,8) = 'sandbox_')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Independent payout simulation - no real funds';

CREATE TABLE IF NOT EXISTS payout_sandbox_event (
  id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
  sandbox_id VARCHAR(44) NOT NULL,
  request_id CHAR(36) NOT NULL,
  request_hash CHAR(64) NOT NULL,
  stage VARCHAR(24) NOT NULL,
  action VARCHAR(16) NOT NULL,
  from_status VARCHAR(24) NOT NULL,
  to_status VARCHAR(24) NOT NULL,
  outcome VARCHAR(16) NOT NULL DEFAULT '',
  version BIGINT NOT NULL,
  attempt INT NOT NULL,
  actor_id VARCHAR(32) NOT NULL,
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY payout_sandbox_event_request (sandbox_id,request_id,stage),
  UNIQUE KEY payout_sandbox_event_version (sandbox_id,version),
  KEY payout_sandbox_events (sandbox_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Append-only payout simulation events';
