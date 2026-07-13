-- Requirement 3: secure DIY design-square order pricing and creator earnings.
SET NAMES utf8mb4;
SET @schema_name = 'askxuan_diy';

SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='payment_status'), 'SELECT 1', "ALTER TABLE askxuan_diy.diy_order ADD COLUMN payment_status VARCHAR(16) NOT NULL DEFAULT 'pending' AFTER status");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='source'), 'SELECT 1', "ALTER TABLE askxuan_diy.diy_order ADD COLUMN source VARCHAR(16) NOT NULL DEFAULT 'custom' AFTER address_id");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='creator_id'), 'SELECT 1', "ALTER TABLE askxuan_diy.diy_order ADD COLUMN creator_id VARCHAR(64) NOT NULL DEFAULT '' AFTER source");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='creator_share_rate'), 'SELECT 1', 'ALTER TABLE askxuan_diy.diy_order ADD COLUMN creator_share_rate DECIMAL(7,6) NOT NULL DEFAULT 0 AFTER creator_id');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='original_material_fee'), 'SELECT 1', 'ALTER TABLE askxuan_diy.diy_order ADD COLUMN original_material_fee DECIMAL(10,2) NOT NULL DEFAULT 0.00 AFTER creator_share_rate');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='price_changed'), 'SELECT 1', 'ALTER TABLE askxuan_diy.diy_order ADD COLUMN price_changed TINYINT NOT NULL DEFAULT 0 AFTER original_material_fee');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='design_snapshot'), 'SELECT 1', 'ALTER TABLE askxuan_diy.diy_order ADD COLUMN design_snapshot LONGTEXT AFTER price_changed');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order' AND column_name='pricing_snapshot'), 'SELECT 1', 'ALTER TABLE askxuan_diy.diy_order ADD COLUMN pricing_snapshot LONGTEXT AFTER design_snapshot');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='diy_order_item' AND column_name='sku_id'), 'SELECT 1', 'ALTER TABLE askxuan_diy.diy_order_item ADD COLUMN sku_id BIGINT NOT NULL DEFAULT 0 AFTER material_id');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS askxuan_diy.diy_config (
  config_key VARCHAR(64) NOT NULL,
  config_value VARCHAR(255) NOT NULL DEFAULT '',
  description VARCHAR(255) NOT NULL DEFAULT '',
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (config_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='DIY业务配置';

INSERT INTO askxuan_diy.diy_config(config_key,config_value,description) VALUES
('diy_design_creator_share','0','设计广场作者分成比例，范围0-1，默认0')
ON DUPLICATE KEY UPDATE description=VALUES(description);

CREATE TABLE IF NOT EXISTS askxuan_diy.diy_creator_earning (
  id BIGINT NOT NULL AUTO_INCREMENT,
  earning_no VARCHAR(32) NOT NULL,
  order_id BIGINT NOT NULL,
  order_no VARCHAR(32) NOT NULL,
  design_id BIGINT NOT NULL,
  creator_id VARCHAR(64) NOT NULL,
  payment_no VARCHAR(32) NOT NULL DEFAULT '',
  base_amount DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  share_rate DECIMAL(7,6) NOT NULL DEFAULT 0,
  earning_amount DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_earning_no (earning_no),
  UNIQUE KEY uk_order (order_id),
  KEY idx_creator_status (creator_id,status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设计广场作者收益';

GRANT SELECT ON askxuan_diy.diy_order TO 'payment_user'@'%';
FLUSH PRIVILEGES;
