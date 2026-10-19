-- Additive growth-v1 migration. No historical payments, mock balances or merit
-- placeholders are converted. Item participation and monetary budgets start OFF.
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_settings (
 id TINYINT PRIMARY KEY, enabled TINYINT NOT NULL DEFAULT 1,
 discounts_enabled TINYINT NOT NULL DEFAULT 0, revision BIGINT NOT NULL DEFAULT 1,
 rule_version VARCHAR(32) NOT NULL DEFAULT 'growth-v1', started_at BIGINT NOT NULL,
 updated_by VARCHAR(64) NOT NULL DEFAULT '', updated_at BIGINT NOT NULL
) ENGINE=InnoDB;
INSERT IGNORE INTO askxuan_payment.growth_settings(id,started_at,updated_at) VALUES(1,UNIX_TIMESTAMP(),UNIX_TIMESTAMP());
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_account (
 user_id VARCHAR(64) PRIMARY KEY, total BIGINT NOT NULL DEFAULT 0,
 revision BIGINT NOT NULL DEFAULT 0, badge_visible TINYINT NOT NULL DEFAULT 1,
 updated_at BIGINT NOT NULL, CONSTRAINT growth_total_nonnegative CHECK(total>=0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_event (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, event_key VARCHAR(160) NOT NULL UNIQUE,
 user_id VARCHAR(64) NOT NULL, kind VARCHAR(32) NOT NULL, reference_no VARCHAR(96) NOT NULL,
 original_award BIGINT NOT NULL DEFAULT 0, current_award BIGINT NOT NULL DEFAULT 0,
 applied BIGINT NOT NULL DEFAULT 0, state VARCHAR(24) NOT NULL DEFAULT 'pending',
 reason VARCHAR(64) NOT NULL DEFAULT '', eligible_at BIGINT NOT NULL DEFAULT 0,
 business_day CHAR(10) NOT NULL, business_month CHAR(7) NOT NULL,
 rule_version VARCHAR(32) NOT NULL, created_at BIGINT NOT NULL, updated_at BIGINT NOT NULL,
 KEY growth_event_user(user_id,id), KEY growth_event_due(state,eligible_at,id),
 KEY growth_event_month(user_id,business_month,kind),
 CHECK(original_award>=0 AND current_award>=0 AND applied>=0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_ledger (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, event_key VARCHAR(192) NOT NULL UNIQUE,
 user_id VARCHAR(64) NOT NULL, source_id BIGINT NOT NULL, kind VARCHAR(32) NOT NULL,
 reference_no VARCHAR(96) NOT NULL, delta BIGINT NOT NULL, total_after BIGINT NOT NULL,
 rule_version VARCHAR(32) NOT NULL, created_at BIGINT NOT NULL,
 KEY growth_ledger_user(user_id,id), CHECK(total_after>=0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_note (
 user_id VARCHAR(64) NOT NULL, business_day CHAR(10) NOT NULL,
 action VARCHAR(120) NOT NULL, body TEXT NOT NULL, deleted TINYINT NOT NULL DEFAULT 0,
 updated_at BIGINT NOT NULL, PRIMARY KEY(user_id,business_day)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_content (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, title VARCHAR(120) NOT NULL,
 kind VARCHAR(16) NOT NULL DEFAULT 'article', body MEDIUMTEXT NOT NULL,
 media_url VARCHAR(1000) NOT NULL DEFAULT '', duration_seconds INT NOT NULL DEFAULT 0,
 enabled TINYINT NOT NULL DEFAULT 0, revision BIGINT NOT NULL DEFAULT 1,
 updated_by VARCHAR(64) NOT NULL, created_at BIGINT NOT NULL, updated_at BIGINT NOT NULL
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_learning_session (
 session_id CHAR(32) PRIMARY KEY, user_id VARCHAR(64) NOT NULL, content_id BIGINT NOT NULL,
 content_revision BIGINT NOT NULL, elapsed_seconds INT NOT NULL DEFAULT 0,
 last_seen BIGINT NOT NULL, expires_at BIGINT NOT NULL, completed TINYINT NOT NULL DEFAULT 0,
 KEY growth_learning_user(user_id,content_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_activity (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, title VARCHAR(120) NOT NULL, description TEXT NOT NULL,
 starts_at BIGINT NOT NULL, ends_at BIGINT NOT NULL, enabled TINYINT NOT NULL DEFAULT 0,
 created_by VARCHAR(64) NOT NULL, created_at BIGINT NOT NULL
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_attendance (
 activity_id BIGINT NOT NULL, user_id VARCHAR(64) NOT NULL,
 registered_at BIGINT NOT NULL, verified_at BIGINT NOT NULL DEFAULT 0,
 verified_by VARCHAR(64) NOT NULL DEFAULT '', evidence VARCHAR(255) NOT NULL DEFAULT '',
 PRIMARY KEY(activity_id,user_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_budget (
 kind VARCHAR(16) PRIMARY KEY, total_cents BIGINT NOT NULL DEFAULT 0,
 held_cents BIGINT NOT NULL DEFAULT 0, spent_cents BIGINT NOT NULL DEFAULT 0,
 revision BIGINT NOT NULL DEFAULT 1,
 CHECK(total_cents>=0 AND held_cents>=0 AND spent_cents>=0 AND total_cents>=held_cents+spent_cents)
) ENGINE=InnoDB;
INSERT IGNORE INTO askxuan_payment.growth_budget(kind) VALUES('level'),('coupon');
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_item_policy (
 item_type VARCHAR(24) NOT NULL, item_id VARCHAR(96) NOT NULL,
 title VARCHAR(120) NOT NULL, owner_type VARCHAR(16) NOT NULL DEFAULT 'platform',
 owner_id VARCHAR(64) NOT NULL DEFAULT '', enabled TINYINT NOT NULL DEFAULT 0,
 consent TINYINT NOT NULL DEFAULT 0, cost_cents BIGINT NOT NULL,
 minimum_contribution_cents BIGINT NOT NULL, subsidy_cap_cents BIGINT NOT NULL,
 revision BIGINT NOT NULL DEFAULT 1, updated_at BIGINT NOT NULL,
 PRIMARY KEY(item_type,item_id), KEY growth_policy_owner(owner_type,owner_id),
 CHECK(cost_cents>=0 AND minimum_contribution_cents>=0 AND subsidy_cap_cents>=0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_coupon (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, user_id VARCHAR(64) NOT NULL,
 level TINYINT NOT NULL, minimum_cents BIGINT NOT NULL, value_cents BIGINT NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'pending_budget', expires_at BIGINT NOT NULL DEFAULT 0,
 quote_id CHAR(32) NOT NULL DEFAULT '', issued_at BIGINT NOT NULL DEFAULT 0,
 UNIQUE KEY growth_coupon_lifetime(user_id,level), KEY growth_coupon_status(status,expires_at)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_month_usage (
 user_id VARCHAR(64) NOT NULL, business_month CHAR(7) NOT NULL,
 held_cents BIGINT NOT NULL DEFAULT 0, spent_cents BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY(user_id,business_month),
 CHECK(held_cents>=0 AND spent_cents>=0 AND held_cents+spent_cents<=30000)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_quote (
 quote_id CHAR(32) PRIMARY KEY, user_id VARCHAR(64) NOT NULL,
 order_type VARCHAR(24) NOT NULL, order_no VARCHAR(64) NOT NULL,
 root_checkout_id VARCHAR(96) NOT NULL, rule_version VARCHAR(32) NOT NULL,
 level TINYINT NOT NULL, status VARCHAR(24) NOT NULL DEFAULT 'held',
 selected_kind VARCHAR(16) NOT NULL, coupon_id BIGINT NOT NULL DEFAULT 0,
 gross_cents BIGINT NOT NULL, discount_cents BIGINT NOT NULL, payable_cents BIGINT NOT NULL,
 growth_base_cents BIGINT NOT NULL, goods_net_cents BIGINT NOT NULL,
 business_month CHAR(7) NOT NULL, snapshot_json MEDIUMTEXT NOT NULL,
 expires_at BIGINT NOT NULL, created_at BIGINT NOT NULL, payment_no VARCHAR(64) NOT NULL DEFAULT '',
 refunded_cents BIGINT NOT NULL DEFAULT 0,
 KEY growth_quote_order(order_type,order_no,status), KEY growth_quote_due(status,expires_at),
 CHECK(gross_cents=discount_cents+payable_cents AND discount_cents>=0 AND payable_cents>0)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_transaction (
 payment_id BIGINT PRIMARY KEY, user_id VARCHAR(64) NOT NULL,
 order_type VARCHAR(24) NOT NULL, order_no VARCHAR(64) NOT NULL,
 gross_cents BIGINT NOT NULL, paid_cents BIGINT NOT NULL, growth_base_cents BIGINT NOT NULL,
 quote_id CHAR(32) NOT NULL DEFAULT '', completion_at BIGINT NOT NULL DEFAULT 0,
 refund_cents BIGINT NOT NULL DEFAULT 0, next_check_at BIGINT NOT NULL,
 created_at BIGINT NOT NULL, KEY growth_transaction_scan(next_check_at,payment_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_proposal (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, kind VARCHAR(24) NOT NULL,
 payload_json MEDIUMTEXT NOT NULL, reason VARCHAR(500) NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'draft', created_by VARCHAR(64) NOT NULL,
 approved_by VARCHAR(64) NOT NULL DEFAULT '', created_at BIGINT NOT NULL,
 approved_at BIGINT NOT NULL DEFAULT 0, KEY growth_proposal_status(status,id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_audit (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, operator_id VARCHAR(64) NOT NULL,
 action VARCHAR(40) NOT NULL, reference_no VARCHAR(96) NOT NULL,
 detail_json MEDIUMTEXT NOT NULL, created_at BIGINT NOT NULL,
 KEY growth_audit_ref(reference_no,id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_payment.growth_support (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, user_id VARCHAR(64) NOT NULL,
 request_id VARCHAR(64) NOT NULL, category VARCHAR(24) NOT NULL,
 body TEXT NOT NULL, level TINYINT NOT NULL, urgency TINYINT NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'queued', reply TEXT NOT NULL,
 created_at BIGINT NOT NULL, updated_at BIGINT NOT NULL,
 UNIQUE KEY growth_support_request(user_id,request_id), KEY growth_support_queue(status,urgency,level,created_at)
) ENGINE=InnoDB;
-- Read existing facts, never infer customer identity from provider IDs.
GRANT SELECT ON askxuan_user.user TO 'payment_user'@'%';
GRANT SELECT ON askxuan_order.shop_order_item TO 'payment_user'@'%';
GRANT SELECT ON askxuan_order.return_order TO 'payment_user'@'%';
GRANT SELECT ON askxuan_diy.diy_order_item TO 'payment_user'@'%';
GRANT SELECT ON askxuan_diy.blessing_task TO 'payment_user'@'%';
GRANT SELECT ON askxuan_booking.booking_status_log TO 'payment_user'@'%';
GRANT SELECT ON askxuan_review.review TO 'payment_user'@'%';
GRANT SELECT ON askxuan_payment.growth_quote TO 'diy_user'@'%';
GRANT SELECT ON askxuan_payment.growth_quote TO 'order_user'@'%';
GRANT SELECT ON askxuan_payment.growth_quote TO 'finance_user'@'%';
GRANT SELECT ON askxuan_product.product TO 'payment_user'@'%';
GRANT SELECT ON askxuan_product.product_sku TO 'payment_user'@'%';
GRANT SELECT ON askxuan_diy.material TO 'payment_user'@'%';
GRANT SELECT ON askxuan_diy.material_sku TO 'payment_user'@'%';
GRANT SELECT ON askxuan_temple.temple TO 'payment_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service TO 'payment_user'@'%';
GRANT SELECT ON askxuan_master.master TO 'payment_user'@'%';
GRANT SELECT ON askxuan_master.master_service_tag TO 'payment_user'@'%';
GRANT SELECT ON askxuan_ai.ai_report_product TO 'payment_user'@'%';
