-- Cash ledger: no balances are derived from historical/mock payments.
CREATE TABLE IF NOT EXISTS askxuan_payment.wallet_account (
 user_id VARCHAR(64) PRIMARY KEY,
 available_cents BIGINT NOT NULL DEFAULT 0,
 held_cents BIGINT NOT NULL DEFAULT 0,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 CONSTRAINT wallet_nonnegative CHECK (available_cents>=0 AND held_cents>=0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.wallet_ledger (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 user_id VARCHAR(64) NOT NULL,
 event_key VARCHAR(128) NOT NULL UNIQUE,
 kind VARCHAR(32) NOT NULL,
 reference_no VARCHAR(64) NOT NULL,
 available_delta BIGINT NOT NULL,
 held_delta BIGINT NOT NULL DEFAULT 0,
 available_after BIGINT NOT NULL,
 held_after BIGINT NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY wallet_user_time(user_id,id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.wallet_recharge (
 recharge_no VARCHAR(32) PRIMARY KEY,
 user_id VARCHAR(64) NOT NULL,
 request_id VARCHAR(64) NOT NULL,
 channel VARCHAR(16) NOT NULL,
 amount_cents BIGINT NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'pending',
 trade_no VARCHAR(96) NULL,
 pay_url TEXT NOT NULL,
 refund_no VARCHAR(32) NOT NULL DEFAULT '',
 refund_cents BIGINT NOT NULL DEFAULT 0,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 next_check_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY wallet_request(user_id,request_id),
 UNIQUE KEY wallet_provider_trade(channel,trade_no),
 KEY wallet_reconcile(status,next_check_at),
 CONSTRAINT wallet_recharge_positive CHECK(amount_cents>0 AND refund_cents>=0 AND refund_cents<=amount_cents)
) ENGINE=InnoDB;
-- All databases are on the same MySQL instance. Limited column updates allow
-- atomic order acceptance + cash debit; workers still own fulfillment.
GRANT SELECT, UPDATE(status) ON askxuan_order.shop_order TO 'payment_user'@'%';
GRANT SELECT, UPDATE(payment_status) ON askxuan_diy.diy_order TO 'payment_user'@'%';
GRANT SELECT, UPDATE(status,payment_no,payment_channel,payment_status) ON askxuan_booking.booking TO 'payment_user'@'%';
GRANT SELECT, INSERT ON askxuan_booking.booking_status_log TO 'payment_user'@'%';
GRANT SELECT, UPDATE(status,payment_no,payment_channel,payment_status,valid_from,expires_at) ON askxuan_booking.consultation_order TO 'payment_user'@'%';
GRANT SELECT ON askxuan_ai.ai_report TO 'payment_user'@'%';
