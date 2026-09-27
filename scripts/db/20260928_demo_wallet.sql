-- Independent test funds. Never migrate these balances into cash accounts.
-- Cash ledger: no balances are derived from historical/mock payments.
CREATE TABLE IF NOT EXISTS askxuan_payment.demo_wallet_account (
 user_id VARCHAR(64) PRIMARY KEY,
 available_cents BIGINT NOT NULL DEFAULT 0,
 held_cents BIGINT NOT NULL DEFAULT 0,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 CONSTRAINT demo_wallet_nonnegative CHECK (available_cents>=0 AND held_cents>=0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.demo_wallet_ledger (
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
 KEY demo_wallet_user_time(user_id,id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.demo_wallet_recharge (
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
 UNIQUE KEY demo_wallet_request(user_id,request_id),
 UNIQUE KEY demo_wallet_provider_trade(channel,trade_no),
 KEY demo_wallet_reconcile(status,next_check_at),
 CONSTRAINT demo_wallet_recharge_positive CHECK(amount_cents>0 AND refund_cents>=0 AND refund_cents<=amount_cents)
) ENGINE=InnoDB;

CREATE TABLE IF NOT EXISTS askxuan_finance.demo_finance_transaction LIKE askxuan_finance.finance_transaction;
CREATE TABLE IF NOT EXISTS askxuan_finance.demo_finance_ledger_entry LIKE askxuan_finance.finance_ledger_entry;
CREATE TABLE IF NOT EXISTS askxuan_finance.demo_finance_log LIKE askxuan_finance.finance_log;
CREATE TABLE IF NOT EXISTS askxuan_finance.demo_settlement LIKE askxuan_finance.settlement;
CREATE TABLE IF NOT EXISTS askxuan_finance.demo_withdrawal LIKE askxuan_finance.withdrawal;
GRANT SELECT ON askxuan_payment.payment TO 'finance_user'@'%';
GRANT SELECT ON askxuan_booking.consultation_order TO 'finance_user'@'%';

GRANT SELECT ON askxuan_finance.demo_settlement TO 'payment_user'@'%';
