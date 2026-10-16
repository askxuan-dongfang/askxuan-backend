-- Additive multi-prize and encrypted digital inventory. No production examples.
USE askxuan_marketing;
CREATE TABLE IF NOT EXISTS reward_prize (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, campaign_id BIGINT NOT NULL,
 position INT NOT NULL DEFAULT 0, name VARCHAR(120) NOT NULL,
 kind VARCHAR(16) NOT NULL, image VARCHAR(2000) NOT NULL DEFAULT '',
 value BIGINT NOT NULL, quantity INT NOT NULL, awarded_count INT NOT NULL DEFAULT 0,
 platform VARCHAR(80) NOT NULL DEFAULT '', instructions TEXT NOT NULL,
 expires_at BIGINT NOT NULL DEFAULT 0, redemption_url VARCHAR(2000) NOT NULL DEFAULT '',
 INDEX idx_reward_prize_campaign(campaign_id,position),
 FOREIGN KEY(campaign_id) REFERENCES reward_campaign(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS reward_stock (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, prize_id BIGINT NOT NULL,
 credential BLOB NOT NULL, credential_digest CHAR(64) NOT NULL,
 order_id BIGINT NULL, created_at BIGINT NOT NULL,
 UNIQUE KEY uk_reward_stock_digest(credential_digest),
 UNIQUE KEY uk_reward_stock_order(order_id), INDEX idx_reward_stock_available(prize_id,order_id,id),
 FOREIGN KEY(prize_id) REFERENCES reward_prize(id),
 FOREIGN KEY(order_id) REFERENCES reward_order(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_entry' AND column_name='prize_id')=0,'ALTER TABLE reward_entry ADD COLUMN prize_id BIGINT NOT NULL DEFAULT 0','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_entry' AND column_name='prize_name')=0,'ALTER TABLE reward_entry ADD COLUMN prize_name VARCHAR(120) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_order' AND column_name='prize_id')=0,'ALTER TABLE reward_order ADD COLUMN prize_id BIGINT NOT NULL DEFAULT 0','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_order' AND column_name='prize_kind')=0,'ALTER TABLE reward_order ADD COLUMN prize_kind VARCHAR(16) NOT NULL DEFAULT ''physical''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_order' AND column_name='platform')=0,'ALTER TABLE reward_order ADD COLUMN platform VARCHAR(80) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_order' AND column_name='instructions')=0,'ALTER TABLE reward_order ADD COLUMN instructions VARCHAR(2000) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_order' AND column_name='expires_at')=0,'ALTER TABLE reward_order ADD COLUMN expires_at BIGINT NOT NULL DEFAULT 0','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_order' AND column_name='redemption_url')=0,'ALTER TABLE reward_order ADD COLUMN redemption_url VARCHAR(2000) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;

-- Older binaries must not award an unspecified prize in a new multi-prize campaign.
DROP TRIGGER IF EXISTS reward_entry_require_prize;
CREATE TRIGGER reward_entry_require_prize BEFORE INSERT ON reward_entry FOR EACH ROW SET NEW.prize_id=IF(NEW.outcome='won' AND EXISTS(SELECT 1 FROM reward_prize WHERE campaign_id=NEW.campaign_id) AND NOT EXISTS(SELECT 1 FROM reward_prize WHERE id=NEW.prize_id AND campaign_id=NEW.campaign_id),NULL,NEW.prize_id);
