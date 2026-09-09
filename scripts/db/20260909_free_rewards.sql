-- Platform-funded rewards. No points, payment or growth tables are read or written.
USE askxuan_marketing;
CREATE TABLE IF NOT EXISTS reward_campaign (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 title VARCHAR(120) NOT NULL, kind VARCHAR(16) NOT NULL,
 prize_name VARCHAR(120) NOT NULL, image VARCHAR(1000) NOT NULL DEFAULT '',
 description TEXT NOT NULL, rules TEXT NOT NULL,
 prize_value BIGINT NOT NULL, budget BIGINT NOT NULL,
 prize_quantity INT NOT NULL, capacity INT NOT NULL,
 participant_count INT NOT NULL DEFAULT 0, awarded_count INT NOT NULL DEFAULT 0,
 starts_at BIGINT NOT NULL, ends_at BIGINT NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'draft', version INT NOT NULL DEFAULT 1,
 published_at BIGINT NOT NULL DEFAULT 0, drawn_at BIGINT NOT NULL DEFAULT 0, pool_digest VARCHAR(64) NOT NULL DEFAULT '',
 announcement TEXT NOT NULL, created_at BIGINT NOT NULL,
 INDEX idx_reward_due(status, ends_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS reward_entry (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, campaign_id BIGINT NOT NULL,
 user_id VARCHAR(64) NOT NULL, code VARCHAR(48) NOT NULL,
 outcome VARCHAR(16) NOT NULL DEFAULT 'pending', created_at BIGINT NOT NULL,
 UNIQUE KEY uk_reward_user(campaign_id,user_id), UNIQUE KEY uk_reward_code(code),
 INDEX idx_reward_user(user_id,id),
 FOREIGN KEY(campaign_id) REFERENCES reward_campaign(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS reward_order (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, campaign_id BIGINT NOT NULL,
 entry_id BIGINT NOT NULL, user_id VARCHAR(64) NOT NULL,
 prize_name VARCHAR(120) NOT NULL, code VARCHAR(48) NOT NULL,
 status VARCHAR(24) NOT NULL DEFAULT 'awaiting_address',
 receiver VARCHAR(80) NOT NULL DEFAULT '', mobile VARCHAR(32) NOT NULL DEFAULT '', address VARCHAR(600) NOT NULL DEFAULT '',
 carrier VARCHAR(80) NOT NULL DEFAULT '', tracking_no VARCHAR(100) NOT NULL DEFAULT '',
 created_at BIGINT NOT NULL, claimed_at BIGINT NOT NULL DEFAULT 0,
 shipped_at BIGINT NOT NULL DEFAULT 0, completed_at BIGINT NOT NULL DEFAULT 0,
 UNIQUE KEY uk_reward_order_entry(entry_id), INDEX idx_reward_order_user(user_id,id),
 FOREIGN KEY(entry_id) REFERENCES reward_entry(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS reward_audit (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, campaign_id BIGINT NOT NULL, order_id BIGINT NOT NULL DEFAULT 0,
 actor VARCHAR(64) NOT NULL, action VARCHAR(32) NOT NULL, detail TEXT NOT NULL, created_at BIGINT NOT NULL,
 INDEX idx_reward_audit_campaign(campaign_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
