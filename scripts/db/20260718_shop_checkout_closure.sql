-- Authoritative shop pricing, stock reservation and idempotent order creation.
-- Safe to run repeatedly on an existing local database.
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS askxuan_product.product_stock_reservation (
  id BIGINT NOT NULL AUTO_INCREMENT,
  request_id VARCHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'reserved',
  snapshot JSON NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_request_id (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @add_order_request_id = IF(
  EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema='askxuan_order' AND table_name='shop_order' AND column_name='request_id'
  ),
  'DO 1',
  'ALTER TABLE askxuan_order.shop_order ADD COLUMN request_id VARCHAR(64) NULL AFTER order_no'
);
PREPARE add_order_request_id_stmt FROM @add_order_request_id;
EXECUTE add_order_request_id_stmt;
DEALLOCATE PREPARE add_order_request_id_stmt;

SET @add_order_request_key = IF(
  EXISTS(
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema='askxuan_order' AND table_name='shop_order' AND index_name='uk_request_id'
  ),
  'DO 1',
  'ALTER TABLE askxuan_order.shop_order ADD UNIQUE KEY uk_request_id (request_id)'
);
PREPARE add_order_request_key_stmt FROM @add_order_request_key;
EXECUTE add_order_request_key_stmt;
DEALLOCATE PREPARE add_order_request_key_stmt;
