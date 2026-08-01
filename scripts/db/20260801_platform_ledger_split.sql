SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS askxuan_finance.finance_transaction (
  id BIGINT NOT NULL AUTO_INCREMENT,
  transaction_no VARCHAR(64) NOT NULL,
  source_type VARCHAR(32) NOT NULL,
  source_no VARCHAR(64) NOT NULL,
  payment_no VARCHAR(64) NOT NULL DEFAULT '',
  event_type VARCHAR(32) NOT NULL,
  total_amount DECIMAL(12,2) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'posted',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_transaction_no (transaction_no),
  UNIQUE KEY uk_source_event (source_type,source_no,event_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台总账事务';

CREATE TABLE IF NOT EXISTS askxuan_finance.finance_ledger_entry (
  id BIGINT NOT NULL AUTO_INCREMENT,
  transaction_id BIGINT NOT NULL,
  account_code VARCHAR(48) NOT NULL,
  target_id VARCHAR(32) NOT NULL DEFAULT '',
  direction VARCHAR(8) NOT NULL,
  amount DECIMAL(12,2) NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_transaction_account (transaction_id,account_code,target_id,direction),
  KEY idx_account_target (account_code,target_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='平台总账借贷分录';

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='askxuan_finance' AND table_name='settlement' AND column_name='source_type'
), 'SELECT 1', 'ALTER TABLE askxuan_finance.settlement ADD COLUMN source_type VARCHAR(32) NOT NULL DEFAULT "" AFTER status');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='askxuan_finance' AND table_name='settlement' AND column_name='source_no'
), 'SELECT 1', 'ALTER TABLE askxuan_finance.settlement ADD COLUMN source_no VARCHAR(64) NOT NULL DEFAULT "" AFTER source_type');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.statistics
  WHERE table_schema='askxuan_finance' AND table_name='settlement' AND index_name='uk_settlement_source'
), 'SELECT 1', 'ALTER TABLE askxuan_finance.settlement ADD UNIQUE KEY uk_settlement_source (source_type,source_no,settle_type,target_id)');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
