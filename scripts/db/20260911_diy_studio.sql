-- Re-runnable additive DIY migration. Back up askxuan_diy before release.
USE askxuan_diy;
SET NAMES utf8mb4;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='diy_design' AND COLUMN_NAME='revision'), 'SELECT 1', 'ALTER TABLE diy_design ADD COLUMN revision BIGINT NOT NULL DEFAULT 1');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='diy_design' AND COLUMN_NAME='source_design_id'), 'SELECT 1', 'ALTER TABLE diy_design ADD COLUMN source_design_id BIGINT NOT NULL DEFAULT 0');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='diy_design' AND COLUMN_NAME='description'), 'SELECT 1', 'ALTER TABLE diy_design ADD COLUMN description VARCHAR(600) NOT NULL DEFAULT ''''');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='material' AND COLUMN_NAME='render_assets'), 'SELECT 1', 'ALTER TABLE material ADD COLUMN render_assets TEXT NULL');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
ALTER TABLE diy_design MODIFY COLUMN design_data MEDIUMTEXT;
SET @diy_ddl = IF(EXISTS(SELECT 1 FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='diy_design' AND INDEX_NAME='idx_design_public_updated'), 'SELECT 1', 'CREATE INDEX idx_design_public_updated ON diy_design(status,update_time,id)');
PREPARE diy_stmt FROM @diy_ddl; EXECUTE diy_stmt; DEALLOCATE PREPARE diy_stmt;
