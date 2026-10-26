-- Cash-only AI report sales. Does not delete/reprice historical reports or ledgers.
SET NAMES utf8mb4;
SET @q=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_ai' AND table_name='ai_report_product' AND column_name='price_revision')=0,
 'ALTER TABLE askxuan_ai.ai_report_product ADD COLUMN price_revision BIGINT NOT NULL DEFAULT 1','SELECT 1');
PREPARE stmt FROM @q; EXECUTE stmt; DEALLOCATE PREPARE stmt;
ALTER TABLE askxuan_ai.ai_report_product ALTER COLUMN price_cents SET DEFAULT 990;
ALTER TABLE askxuan_ai.ai_report_product ALTER COLUMN points_price SET DEFAULT 0;
CREATE TABLE IF NOT EXISTS askxuan_ai.ai_report_price_audit (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, product_code VARCHAR(32) NOT NULL, actor_id VARCHAR(64) NOT NULL,
 old_price_cents BIGINT NOT NULL,new_price_cents BIGINT NOT NULL,revision BIGINT NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY product_revision(product_code,revision)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
-- Entitlement checks preserve real historical purchases and frozen discounts.
GRANT SELECT ON askxuan_payment.refund TO 'ai_user'@'%';
GRANT SELECT ON askxuan_payment.growth_transaction TO 'ai_user'@'%';
GRANT SELECT ON askxuan_payment.demo_wallet_ledger TO 'ai_user'@'%';
-- Receipt only; the operator tool performs no cleanup unless explicitly applied.
CREATE TABLE IF NOT EXISTS askxuan_payment.ai_report_points_retirement (
 batch_id VARCHAR(48) PRIMARY KEY,manifest_sha256 CHAR(64) NOT NULL,status VARCHAR(20) NOT NULL,
 reports BIGINT NOT NULL,ledger_entries BIGINT NOT NULL,points_refunded BIGINT NOT NULL,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
