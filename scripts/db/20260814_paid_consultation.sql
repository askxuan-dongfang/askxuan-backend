SET NAMES utf8mb4;

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='askxuan_master' AND table_name='master' AND column_name='consult_enabled'
), 'SELECT 1', 'ALTER TABLE askxuan_master.master ADD COLUMN consult_enabled TINYINT(1) NOT NULL DEFAULT 1 AFTER rating');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='askxuan_master' AND table_name='master' AND column_name='consult_fee'
), 'SELECT 1', 'ALTER TABLE askxuan_master.master ADD COLUMN consult_fee DECIMAL(10,2) NOT NULL DEFAULT 39.00 AFTER consult_enabled');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='askxuan_master' AND table_name='master' AND column_name='consult_valid_hours'
), 'SELECT 1', 'ALTER TABLE askxuan_master.master ADD COLUMN consult_valid_hours INT NOT NULL DEFAULT 72 AFTER consult_fee');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='askxuan_master' AND table_name='master' AND column_name='consult_response_minutes'
), 'SELECT 1', 'ALTER TABLE askxuan_master.master ADD COLUMN consult_response_minutes INT NOT NULL DEFAULT 30 AFTER consult_valid_hours');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE askxuan_master.master SET
  consult_enabled=IF(code='M005',0,1),
  consult_fee=CASE code WHEN 'M002' THEN 49 WHEN 'M004' THEN 59 WHEN 'M006' THEN 49 WHEN 'M008' THEN 59 WHEN 'M009' THEN 49 ELSE 39 END,
  consult_valid_hours=72,
  consult_response_minutes=CASE WHEN code IN ('M004','M008') THEN 45 ELSE 30 END;

CREATE TABLE IF NOT EXISTS askxuan_booking.consultation_order (
  order_no VARCHAR(32) NOT NULL,
  request_id VARCHAR(64) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  master_code VARCHAR(16) NOT NULL,
  master_name VARCHAR(64) NOT NULL,
  temple_code VARCHAR(16) NOT NULL DEFAULT '',
  temple_name VARCHAR(64) NOT NULL DEFAULT '',
  consult_fee DECIMAL(10,2) NOT NULL,
  valid_hours INT NOT NULL,
  response_minutes INT NOT NULL,
  question VARCHAR(500) NOT NULL DEFAULT '',
  price_snapshot JSON DEFAULT NULL,
  payment_no VARCHAR(64) NOT NULL DEFAULT '',
  payment_channel VARCHAR(32) NOT NULL DEFAULT '',
  payment_status VARCHAR(32) NOT NULL DEFAULT 'pending',
  status VARCHAR(32) NOT NULL DEFAULT 'pending_payment',
  valid_from DATETIME DEFAULT NULL,
  expires_at DATETIME DEFAULT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (order_no),
  UNIQUE KEY uk_consult_request (user_id,request_id),
  KEY idx_consult_user (user_id,status,create_time),
  KEY idx_consult_master (master_code,status,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='独立即时文字咨询订单';

SET @sql = IF(EXISTS(
  SELECT 1 FROM information_schema.columns
  WHERE table_schema='askxuan_booking' AND table_name='booking_chat_message' AND column_name='source_type'
), 'SELECT 1', 'ALTER TABLE askxuan_booking.booking_chat_message ADD COLUMN source_type VARCHAR(32) NOT NULL DEFAULT "booking" AFTER booking_id');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

INSERT INTO askxuan_finance.commission_config (biz_type,rate,description,update_time)
VALUES ('consultation',0.2000,'即时文字咨询平台抽成20%',NOW())
ON DUPLICATE KEY UPDATE description=VALUES(description);
