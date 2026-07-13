-- Requirement 1: persistent AI conversations and provider status. Safe to run repeatedly.
SET NAMES utf8mb4;
SET @schema_name = 'askxuan_ai';
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_session' AND column_name='title'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_session ADD COLUMN title VARCHAR(100) NOT NULL DEFAULT '新对话' AFTER skill_code");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='status'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'completed' AFTER tokens");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='error_message'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN error_message VARCHAR(255) NOT NULL DEFAULT '' AFTER status");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='retry_count'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN retry_count INT NOT NULL DEFAULT 0 AFTER error_message");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=@schema_name AND table_name='ai_message' AND index_name='idx_message_status'), 'SELECT 1', 'CREATE INDEX idx_message_status ON askxuan_ai.ai_message(status)');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

INSERT INTO askxuan_ai.ai_skill(code,name,description,icon,prompt_template,status) VALUES
('general','直接问事','不限定术数方向的日常问事入口','/icons/general.png','你是问玄东方的AI问事助手。请以审慎、尊重的方式回应，不把玄学内容表述为确定事实，也不替代医疗、法律或财务专业建议。','enabled')
ON DUPLICATE KEY UPDATE name=VALUES(name),description=VALUES(description),icon=VALUES(icon),prompt_template=VALUES(prompt_template),status=VALUES(status);
UPDATE askxuan_ai.ai_session SET title='新对话' WHERE title='';
UPDATE askxuan_ai.ai_message SET status='completed' WHERE status='';
