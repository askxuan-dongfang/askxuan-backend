-- AI 专用智能体产品化：动态技能、结构化输入、流式状态、用户额度与成本账。
SET @schema_name = 'askxuan_ai';

SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='category'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN category VARCHAR(32) NOT NULL DEFAULT 'divination' AFTER code");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='version'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN version VARCHAR(32) NOT NULL DEFAULT '1.0.0' AFTER name");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='source_type'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN source_type VARCHAR(32) NOT NULL DEFAULT 'builtin' AFTER icon");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='source_ref'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN source_ref VARCHAR(255) NOT NULL DEFAULT '' AFTER source_type");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='input_schema'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN input_schema JSON NULL AFTER prompt_template");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='capabilities'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN capabilities JSON NULL AFTER input_schema");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='tool_config'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN tool_config JSON NULL AFTER capabilities");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='risk_level'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN risk_level VARCHAR(16) NOT NULL DEFAULT 'medium' AFTER tool_config");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='sort_order'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN sort_order INT NOT NULL DEFAULT 0 AFTER risk_level");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='input_json'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN input_json JSON NULL AFTER content");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='prompt_tokens'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN prompt_tokens INT NOT NULL DEFAULT 0 AFTER tokens");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='completion_tokens'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN completion_tokens INT NOT NULL DEFAULT 0 AFTER prompt_tokens");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='provider'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN provider VARCHAR(32) NOT NULL DEFAULT '' AFTER completion_tokens");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='model'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN model VARCHAR(128) NOT NULL DEFAULT '' AFTER provider");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='cost_micros'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN cost_micros BIGINT NOT NULL DEFAULT 0 AFTER model");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='finish_reason'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN finish_reason VARCHAR(32) NOT NULL DEFAULT '' AFTER cost_micros");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS askxuan_ai.ai_usage_counter (
  user_id VARCHAR(64) NOT NULL,
  bucket_type VARCHAR(16) NOT NULL COMMENT 'minute/day',
  bucket_start DATETIME NOT NULL,
  request_count INT NOT NULL DEFAULT 0,
  total_tokens BIGINT NOT NULL DEFAULT 0,
  cost_micros BIGINT NOT NULL DEFAULT 0,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id,bucket_type,bucket_start),
  KEY idx_bucket (bucket_type,bucket_start)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI用户额度计数';

CREATE TABLE IF NOT EXISTS askxuan_ai.ai_usage_log (
  id BIGINT NOT NULL AUTO_INCREMENT,
  user_id VARCHAR(64) NOT NULL,
  session_id BIGINT NOT NULL,
  message_id BIGINT NOT NULL,
  skill_code VARCHAR(32) NOT NULL,
  provider VARCHAR(32) NOT NULL,
  model VARCHAR(128) NOT NULL DEFAULT '',
  prompt_tokens INT NOT NULL DEFAULT 0,
  completion_tokens INT NOT NULL DEFAULT 0,
  total_tokens INT NOT NULL DEFAULT 0,
  cost_micros BIGINT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL COMMENT 'completed/failed/blocked',
  latency_ms INT NOT NULL DEFAULT 0,
  error_message VARCHAR(255) NOT NULL DEFAULT '',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_message (message_id),
  KEY idx_user_time (user_id,create_time),
  KEY idx_provider_time (provider,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI调用与成本明细';

-- 受控技能目录。source_ref 记录本地 ai-module-skills 来源，不在运行时执行仓库代码。
UPDATE askxuan_ai.ai_skill SET category='assistant',version='1.0.0',source_type='builtin',source_ref='askxuan/general',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY()), capabilities=JSON_ARRAY('chat','stream'), tool_config=JSON_OBJECT('enabled',FALSE),risk_level='low',sort_order=0
 WHERE code='general';
UPDATE askxuan_ai.ai_skill SET category='divination',version='1.0.0',source_type='skill_repo',source_ref='ai-module-skills/bazi-skill@112a5d8',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(
   JSON_OBJECT('key','birthDate','label','出生日期','type','date','required',TRUE),
   JSON_OBJECT('key','birthTime','label','出生时间','type','time','required',FALSE),
   JSON_OBJECT('key','calendarType','label','历法','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','solar','label','公历'),JSON_OBJECT('value','lunar','label','农历'))),
   JSON_OBJECT('key','gender','label','性别','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','male','label','男'),JSON_OBJECT('value','female','label','女'),JSON_OBJECT('value','other','label','其他'))),
   JSON_OBJECT('key','birthplace','label','出生地','type','text','required',TRUE,'placeholder','省 / 市 / 区县')
 )), capabilities=JSON_ARRAY('chat','stream','structured_input','mcp'),tool_config=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','bazi'),risk_level='medium',sort_order=10
 WHERE code='bazi';
UPDATE askxuan_ai.ai_skill SET category='divination',version='1.0.0',source_type='skill_repo',source_ref='ai-module-skills/yinyuan-skills@b091c88',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(
   JSON_OBJECT('key','mode','label','测算方式','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','personal','label','个人姻缘'),JSON_OBJECT('value','matching','label','双方合盘'))),
   JSON_OBJECT('key','birthDate','label','你的出生日期','type','date','required',FALSE),
   JSON_OBJECT('key','partnerBirthDate','label','对方出生日期','type','date','required',FALSE)
 )),capabilities=JSON_ARRAY('chat','stream','structured_input'),tool_config=JSON_OBJECT('enabled',FALSE),risk_level='medium',sort_order=20
 WHERE code='marriage';
UPDATE askxuan_ai.ai_skill SET category='divination',version='1.0.0',source_type='skill_repo',source_ref='ai-module-skills/tarot-skill@2d15f52',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','spread','label','牌阵','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','single','label','单牌'),JSON_OBJECT('value','three','label','三牌阵'),JSON_OBJECT('value','love','label','爱情牌阵'))))),
 capabilities=JSON_ARRAY('chat','stream','structured_input','mcp'),tool_config=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','tarot'),risk_level='medium',sort_order=30
 WHERE code='tarot';
UPDATE askxuan_ai.ai_skill SET category='divination',version='1.0.0',source_type='skill_repo',source_ref='ai-module-skills/fengshui.skill@dc6ffb4',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','scene','label','场景','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','home','label','住宅'),JSON_OBJECT('value','office','label','办公'),JSON_OBJECT('value','shop','label','商铺'))),JSON_OBJECT('key','location','label','地点','type','text','required',FALSE),JSON_OBJECT('key','orientation','label','朝向','type','text','required',FALSE))),
 capabilities=JSON_ARRAY('chat','stream','structured_input'),tool_config=JSON_OBJECT('enabled',FALSE),risk_level='medium',sort_order=40
 WHERE code='fengshui';
UPDATE askxuan_ai.ai_skill SET category='divination',version='1.0.0',source_type='skill_repo',source_ref='ai-module-skills/Numerologist_skills/qimen-dunjia@ea28c3f',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','eventTime','label','起局时间','type','datetime','required',TRUE),JSON_OBJECT('key','location','label','所在地','type','text','required',TRUE))),
 capabilities=JSON_ARRAY('chat','stream','structured_input','mcp'),tool_config=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','qimen'),risk_level='medium',sort_order=50
 WHERE code='qimen';
UPDATE askxuan_ai.ai_skill SET category='divination',version='1.0.0',source_type='skill_repo',source_ref='ai-module-skills/Numerologist_skills/ziwei-doushu@ea28c3f',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','birthDate','label','出生日期','type','date','required',TRUE),JSON_OBJECT('key','birthTime','label','出生时间','type','time','required',TRUE),JSON_OBJECT('key','calendarType','label','历法','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','solar','label','公历'),JSON_OBJECT('value','lunar','label','农历'))),JSON_OBJECT('key','gender','label','性别','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','male','label','男'),JSON_OBJECT('value','female','label','女'))),JSON_OBJECT('key','birthplace','label','出生地','type','text','required',TRUE))),
 capabilities=JSON_ARRAY('chat','stream','structured_input','mcp'),tool_config=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','ziwei'),risk_level='medium',sort_order=60
 WHERE code='ziwei';
UPDATE askxuan_ai.ai_skill SET category='divination',version='1.0.0',source_type='skill_repo',source_ref='ai-module-skills/taibu@4ef4d7a',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','method','label','起卦方式','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','time','label','时间起卦'),JSON_OBJECT('value','numbers','label','数字起卦'),JSON_OBJECT('value','manual','label','手动起卦'))))),
 capabilities=JSON_ARRAY('chat','stream','structured_input','mcp'),tool_config=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','liuyao'),risk_level='medium',sort_order=70
 WHERE code='liuyao';
