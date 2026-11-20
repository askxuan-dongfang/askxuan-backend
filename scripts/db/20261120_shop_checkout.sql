USE askxuan_order;

SET @shop_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='shop_order' AND column_name='freight_amount'),
  'SELECT 1',
  'ALTER TABLE shop_order ADD COLUMN freight_amount DECIMAL(10,2) NOT NULL DEFAULT 0.00'
);
PREPARE shop_stmt FROM @shop_ddl;
EXECUTE shop_stmt;
DEALLOCATE PREPARE shop_stmt;

SET @shop_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='shop_order' AND column_name='payment_expire_time'),
  'SELECT 1',
  'ALTER TABLE shop_order ADD COLUMN payment_expire_time DATETIME NULL'
);
PREPARE shop_stmt FROM @shop_ddl;
EXECUTE shop_stmt;
DEALLOCATE PREPARE shop_stmt;

SET @shop_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='shop_order' AND column_name='stock_release_pending'),
  'SELECT 1',
  'ALTER TABLE shop_order ADD COLUMN stock_release_pending TINYINT NOT NULL DEFAULT 0'
);
PREPARE shop_stmt FROM @shop_ddl;
EXECUTE shop_stmt;
DEALLOCATE PREPARE shop_stmt;

SET @shop_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='shop_order' AND index_name='idx_shop_order_expiry'),
  'SELECT 1',
  'ALTER TABLE shop_order ADD INDEX idx_shop_order_expiry(status,payment_expire_time)'
);
PREPARE shop_stmt FROM @shop_ddl;
EXECUTE shop_stmt;
DEALLOCATE PREPARE shop_stmt;

SET @shop_ddl = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='shop_order' AND index_name='idx_shop_order_stock_release'),
  'SELECT 1',
  'ALTER TABLE shop_order ADD INDEX idx_shop_order_stock_release(stock_release_pending,id)'
);
PREPARE shop_stmt FROM @shop_ddl;
EXECUTE shop_stmt;
DEALLOCATE PREPARE shop_stmt;

GRANT SELECT (id, user_id, province) ON askxuan_user.user_address TO 'order_user'@'%';
GRANT SELECT (id, type, free_shipping, config, status) ON askxuan_logistics.freight_template TO 'order_user'@'%';
GRANT SELECT (order_type, order_no, status) ON askxuan_payment.payment TO 'order_user'@'%';
