-- Per-attempt idempotency for repeat wheel spins. Preserve every old entry.
USE askxuan_marketing;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_campaign' AND column_name='wheel_slots')=0,'ALTER TABLE reward_campaign ADD COLUMN wheel_slots TEXT NULL','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_entry' AND column_name='request_key')=0,'ALTER TABLE reward_entry ADD COLUMN request_key VARCHAR(80) NOT NULL DEFAULT ''legacy''','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='reward_entry' AND index_name='uk_reward_attempt')=0,'ALTER TABLE reward_entry ADD UNIQUE KEY uk_reward_attempt(campaign_id,user_id,request_key)','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='reward_entry' AND index_name='uk_reward_user')>0,'ALTER TABLE reward_entry DROP INDEX uk_reward_user','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
