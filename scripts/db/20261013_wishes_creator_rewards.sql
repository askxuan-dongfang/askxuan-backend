-- Additive migration. Existing earnings are not retroactively funded.
CREATE TABLE IF NOT EXISTS askxuan_product.master_service_intention (
 master_code VARCHAR(64) NOT NULL, service_code VARCHAR(32) NOT NULL, tag_code VARCHAR(32) NOT NULL,
 PRIMARY KEY(master_code,service_code,tag_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_product.intention_migration (version VARCHAR(64) PRIMARY KEY);
INSERT IGNORE INTO askxuan_product.master_service_intention
 SELECT DISTINCT mst.master_code,mst.service_code,it.tag_code
 FROM askxuan_master.master_service_tag mst
 JOIN askxuan_temple.temple_service ts ON ts.service_code=mst.service_code
 JOIN askxuan_temple.temple_service_intent_tag it ON it.temple_service_id=ts.id
 WHERE it.tag_code IN ('peace','wealth','love','career','study','taisui','rite') AND NOT EXISTS(SELECT 1 FROM askxuan_product.intention_migration WHERE version='20261013');
INSERT IGNORE INTO askxuan_product.intention_migration VALUES('20261013');
CREATE TABLE IF NOT EXISTS askxuan_diy.creator_reward_policy (
 id INT PRIMARY KEY, enabled BOOLEAN NOT NULL DEFAULT 0, kind VARCHAR(16) NOT NULL DEFAULT 'cash',
 rate_bps INT NOT NULL DEFAULT 500, cap_cents BIGINT NOT NULL DEFAULT 2000, wait_days INT NOT NULL DEFAULT 7,
 min_withdraw_cents BIGINT NOT NULL DEFAULT 100, coupon_id BIGINT NOT NULL DEFAULT 0,
 revision INT NOT NULL DEFAULT 1, updated_by VARCHAR(64) NOT NULL DEFAULT '', updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT IGNORE INTO askxuan_diy.creator_reward_policy(id,enabled) VALUES(1,1);
CREATE TABLE IF NOT EXISTS askxuan_diy.creator_reward (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, order_id BIGINT NOT NULL, order_no VARCHAR(64) NOT NULL,
 design_id BIGINT NOT NULL, creator_id VARCHAR(64) NOT NULL, kind VARCHAR(16) NOT NULL,
 rate_bps INT NOT NULL, cap_cents BIGINT NOT NULL, wait_days INT NOT NULL, coupon_id BIGINT NOT NULL DEFAULT 0,
 policy_revision INT NOT NULL, base_cents BIGINT NOT NULL, amount_cents BIGINT NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'pending', reason VARCHAR(255) NOT NULL DEFAULT '',
 next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, settled_at DATETIME NULL,
 UNIQUE KEY uk_order(order_id), KEY idx_creator(creator_id,id), KEY idx_status(status,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_diy.creator_cash_account (
 user_id VARCHAR(64) PRIMARY KEY, available_cents BIGINT NOT NULL DEFAULT 0, held_cents BIGINT NOT NULL DEFAULT 0, paid_cents BIGINT NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_diy.creator_cash_ledger (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, user_id VARCHAR(64) NOT NULL, event_key VARCHAR(128) NOT NULL,
 kind VARCHAR(32) NOT NULL, amount_cents BIGINT NOT NULL, held_delta BIGINT NOT NULL DEFAULT 0,
 reference_no VARCHAR(64) NOT NULL, available_after BIGINT NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY uk_event(event_key), KEY idx_user(user_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_diy.creator_withdrawal (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, user_id VARCHAR(64) NOT NULL, request_key VARCHAR(64) NOT NULL,
 amount_cents BIGINT NOT NULL, holder_name VARCHAR(100) NOT NULL, bank_name VARCHAR(150) NOT NULL, account_number VARCHAR(512) NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'pending', transfer_reference VARCHAR(128) NULL,
 note VARCHAR(500) NOT NULL DEFAULT '', operator_id VARCHAR(64) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 UNIQUE KEY uk_request(user_id,request_key), UNIQUE KEY uk_transfer(transfer_reference), KEY idx_status(status,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_diy.creator_reward_audit (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, operator_id VARCHAR(64) NOT NULL, action VARCHAR(32) NOT NULL,
 reference_no VARCHAR(64) NOT NULL, detail TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
GRANT SELECT ON askxuan_payment.payment TO 'diy_user'@'%';
GRANT SELECT ON askxuan_payment.refund TO 'diy_user'@'%';
CREATE TABLE IF NOT EXISTS askxuan_diy.creator_reward_delivery (reward_id BIGINT PRIMARY KEY,coupon_record_id BIGINT NULL) ENGINE=InnoDB;
GRANT SELECT,INSERT,UPDATE ON askxuan_payment.wallet_account TO 'diy_user'@'%';
GRANT SELECT,INSERT ON askxuan_payment.wallet_ledger TO 'diy_user'@'%';
GRANT SELECT,INSERT,UPDATE ON askxuan_marketing.coupon TO 'diy_user'@'%';
GRANT SELECT,INSERT,UPDATE ON askxuan_marketing.coupon_record TO 'diy_user'@'%';

GRANT SELECT,INSERT,DELETE ON askxuan_temple.temple_service_intent_tag TO 'product_user'@'%';
