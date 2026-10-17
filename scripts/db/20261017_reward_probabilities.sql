-- Fixed per-prize probabilities in basis points (100 = 1%). NULL retains legacy rules.
USE askxuan_marketing;
SET @reward_ddl=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='reward_prize' AND column_name='probability_bps')=0,'ALTER TABLE reward_prize ADD COLUMN probability_bps INT NULL DEFAULT NULL','SELECT 1');
PREPARE reward_stmt FROM @reward_ddl;
EXECUTE reward_stmt;
DEALLOCATE PREPARE reward_stmt;
