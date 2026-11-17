-- Independent pool prize limits and draws. Existing single-prize campaigns and entries stay unchanged.
USE askxuan_marketing;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_prize' AND column_name='capacity')=0,'ALTER TABLE reward_prize ADD COLUMN capacity INT NOT NULL DEFAULT 0','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_prize' AND column_name='participant_count')=0,'ALTER TABLE reward_prize ADD COLUMN participant_count INT NOT NULL DEFAULT 0','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_prize' AND column_name='drawn_at')=0,'ALTER TABLE reward_prize ADD COLUMN drawn_at BIGINT NOT NULL DEFAULT 0','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_prize' AND column_name='pool_digest')=0,'ALTER TABLE reward_prize ADD COLUMN pool_digest VARCHAR(64) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_prize' AND column_name='announcement')=0,'ALTER TABLE reward_prize ADD COLUMN announcement VARCHAR(1000) NOT NULL DEFAULT ''''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='reward_entry' AND index_name='idx_reward_entry_prize')=0,'ALTER TABLE reward_entry ADD INDEX idx_reward_entry_prize(campaign_id,prize_id,id)','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;

-- Older binaries must not debit a multi-prize pool entry without a selected prize.
DROP TRIGGER IF EXISTS reward_entry_require_prize;
CREATE TRIGGER reward_entry_require_prize BEFORE INSERT ON reward_entry FOR EACH ROW SET NEW.prize_id=IF((NEW.outcome='won' OR (SELECT kind FROM reward_campaign WHERE id=NEW.campaign_id)='pool') AND EXISTS(SELECT 1 FROM reward_prize WHERE campaign_id=NEW.campaign_id) AND NOT EXISTS(SELECT 1 FROM reward_prize WHERE id=NEW.prize_id AND campaign_id=NEW.campaign_id),NULL,NEW.prize_id);
