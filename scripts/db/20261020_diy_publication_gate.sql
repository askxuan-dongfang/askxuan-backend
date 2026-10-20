-- Additive and re-runnable. Back up askxuan_diy before applying.
-- No legacy work or financial ledger is rewritten/backfilled as qualified.
USE askxuan_diy;
SET NAMES utf8mb4;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='diy_design' AND COLUMN_NAME='publication_id'), 'SELECT 1', 'ALTER TABLE diy_design ADD COLUMN publication_id BIGINT NOT NULL DEFAULT 0');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='diy_design' AND COLUMN_NAME='source_publication_id'), 'SELECT 1', 'ALTER TABLE diy_design ADD COLUMN source_publication_id BIGINT NOT NULL DEFAULT 0');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
CREATE TABLE IF NOT EXISTS diy_design_publication (
 id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY,
 design_id BIGINT NOT NULL,
 design_revision BIGINT NOT NULL,
 creator_id VARCHAR(64) NOT NULL,
 reward_creator_id VARCHAR(64) NOT NULL DEFAULT '',
 reward_publication_id BIGINT NOT NULL DEFAULT 0,
 order_id BIGINT NOT NULL,
 composition_hash CHAR(64) NOT NULL,
 design_snapshot MEDIUMTEXT NOT NULL,
 pricing_snapshot MEDIUMTEXT NOT NULL,
 material_fee DECIMAL(12,2) NOT NULL,
 published_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE KEY uk_design_revision(design_id,design_revision),
 KEY idx_order(order_id),
 KEY idx_creator(creator_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='creator_reward' AND COLUMN_NAME='source_publication_id'), 'SELECT 1', 'ALTER TABLE creator_reward ADD COLUMN source_publication_id BIGINT NOT NULL DEFAULT 0');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
