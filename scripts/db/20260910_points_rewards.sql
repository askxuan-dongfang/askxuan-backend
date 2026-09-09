-- Additive migration: preserve historical free entries as zero, never retroactively debit.
USE askxuan_marketing;
SET @reward_ddl = IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_campaign' AND column_name='points_cost')=0, 'ALTER TABLE reward_campaign ADD COLUMN points_cost BIGINT NOT NULL DEFAULT 0', 'SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl = IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_entry' AND column_name='points_spent')=0, 'ALTER TABLE reward_entry ADD COLUMN points_spent BIGINT NOT NULL DEFAULT 0', 'SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;

-- Block old free-participation binaries after rollback without changing historical rows.
DROP TRIGGER IF EXISTS reward_entry_require_points;
CREATE TRIGGER reward_entry_require_points BEFORE INSERT ON reward_entry FOR EACH ROW SET NEW.points_spent = IF(NEW.points_spent > 0 AND NEW.points_spent = (SELECT points_cost FROM reward_campaign WHERE id=NEW.campaign_id), NEW.points_spent, NULL);
