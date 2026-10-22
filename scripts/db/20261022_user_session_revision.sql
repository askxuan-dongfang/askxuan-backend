-- Additive durable consumer-session revision. Apply BEFORE auth/user/gateway rollout.
-- No credential, user ID, order, or asset changes. Re-running preserves revisions.
USE askxuan_user;
SET @ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='user' AND column_name='authz_version')=0, 'ALTER TABLE `user` ADD COLUMN `authz_version` BIGINT UNSIGNED NOT NULL DEFAULT 0', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
