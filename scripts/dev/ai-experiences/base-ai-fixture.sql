SET NAMES utf8mb4;
USE `askxuan_ai`;

CREATE TABLE IF NOT EXISTS `ai_skill` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(32) NOT NULL COMMENT '技能编码',
  `category` VARCHAR(32) NOT NULL DEFAULT 'divination',
  `name` VARCHAR(64) NOT NULL COMMENT '技能名称',
  `version` VARCHAR(32) NOT NULL DEFAULT '1.0.0',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
  `icon` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '图标',
  `source_type` VARCHAR(32) NOT NULL DEFAULT 'builtin',
  `source_ref` VARCHAR(255) NOT NULL DEFAULT '',
  `prompt_template` TEXT COMMENT '提示词模板',
  `input_schema` JSON NULL,
	`routing_keywords` JSON NULL,
  `capabilities` JSON NULL,
  `tool_config` JSON NULL,
  `risk_level` VARCHAR(16) NOT NULL DEFAULT 'medium',
  `sort_order` INT NOT NULL DEFAULT 0,
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI技能配置';

INSERT INTO `ai_skill` (`code`,`name`,`description`,`icon`,`prompt_template`,`status`,`create_time`) VALUES
('general','直接问事','不限定术数方向的日常问事入口','/icons/general.png','你是问玄东方的AI问事助手。请以审慎、尊重的方式回应，不把玄学内容表述为确定事实，也不替代医疗、法律或财务专业建议。','enabled','2026-07-13 10:00:00'),
('bazi','八字命理','依据生辰八字推演命格运势','/icons/bazi.png','你是一位精通八字命理的师傅，请根据用户八字解答...','enabled','2026-06-28 10:00:00'),
('marriage','姻缘测算','测算姻缘婚恋走势','/icons/marriage.png','你是一位姻缘测算师，请根据用户信息解答感情问题...','enabled','2026-06-28 10:00:00'),
('tarot','塔罗牌','塔罗牌占卜指引','/icons/tarot.png','你是一位塔罗牌占卜师，请为用户抽牌并解读...','enabled','2026-06-28 10:00:00'),
('fengshui','风水分析','居家风水布局建议','/icons/fengshui.png','你是一位风水师，请根据用户描述分析风水布局...','enabled','2026-06-28 10:00:00'),
('qimen','奇门遁甲','奇门遁甲预测决策','/icons/qimen.png','你是一位奇门遁甲大师，请根据用户问题起局预测...','enabled','2026-06-28 10:00:00'),
('ziwei','紫微斗数','紫微斗数命盘解析','/icons/ziwei.png','你是一位紫微斗数师傅，请根据用户命盘解析运势...','enabled','2026-06-28 10:00:00'),
('liuyao','六爻梅花','六爻梅花易数占断','/icons/liuyao.png','你是一位六爻梅花易数师傅，请根据用户问题起卦占断...','enabled','2026-06-28 10:00:00');

UPDATE `ai_skill` SET `category`='assistant',`source_ref`='askxuan/general',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY()),`capabilities`=JSON_ARRAY('chat','stream'),`tool_config`=JSON_OBJECT('enabled',FALSE),`risk_level`='low',`sort_order`=0 WHERE `code`='general';
UPDATE `ai_skill` SET `source_type`='skill_repo',`source_ref`='ai-module-skills/bazi-skill@112a5d8',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','birthDate','label','出生日期','type','date','required',TRUE),JSON_OBJECT('key','birthTime','label','出生时间','type','time','required',FALSE),JSON_OBJECT('key','calendarType','label','历法','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','solar','label','公历'),JSON_OBJECT('value','lunar','label','农历'))),JSON_OBJECT('key','gender','label','性别','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','male','label','男'),JSON_OBJECT('value','female','label','女'),JSON_OBJECT('value','other','label','其他'))),JSON_OBJECT('key','birthplace','label','出生地','type','text','required',TRUE))),`capabilities`=JSON_ARRAY('chat','stream','structured_input','mcp'),`tool_config`=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','bazi'),`sort_order`=10 WHERE `code`='bazi';
UPDATE `ai_skill` SET `source_type`='skill_repo',`source_ref`='ai-module-skills/yinyuan-skills@b091c88',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','mode','label','测算方式','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','personal','label','个人姻缘'),JSON_OBJECT('value','matching','label','双方合盘'))),JSON_OBJECT('key','birthDate','label','你的出生日期','type','date','required',FALSE),JSON_OBJECT('key','partnerBirthDate','label','对方出生日期','type','date','required',FALSE))),`capabilities`=JSON_ARRAY('chat','stream','structured_input'),`tool_config`=JSON_OBJECT('enabled',FALSE),`sort_order`=20 WHERE `code`='marriage';
UPDATE `ai_skill` SET `source_type`='skill_repo',`source_ref`='ai-module-skills/tarot-skill@2d15f52',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','spread','label','牌阵','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','single','label','单牌'),JSON_OBJECT('value','three','label','三牌阵'),JSON_OBJECT('value','love','label','爱情牌阵'))))),`capabilities`=JSON_ARRAY('chat','stream','structured_input','mcp'),`tool_config`=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','tarot'),`sort_order`=30 WHERE `code`='tarot';
UPDATE `ai_skill` SET `source_type`='skill_repo',`source_ref`='ai-module-skills/fengshui.skill@dc6ffb4',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','scene','label','场景','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','home','label','住宅'),JSON_OBJECT('value','office','label','办公'),JSON_OBJECT('value','shop','label','商铺'))),JSON_OBJECT('key','location','label','地点','type','text','required',FALSE),JSON_OBJECT('key','orientation','label','朝向','type','text','required',FALSE))),`capabilities`=JSON_ARRAY('chat','stream','structured_input'),`tool_config`=JSON_OBJECT('enabled',FALSE),`sort_order`=40 WHERE `code`='fengshui';
UPDATE `ai_skill` SET `source_type`='skill_repo',`source_ref`='ai-module-skills/Numerologist_skills/qimen-dunjia@ea28c3f',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','eventTime','label','起局时间','type','datetime','required',TRUE),JSON_OBJECT('key','location','label','所在地','type','text','required',TRUE))),`capabilities`=JSON_ARRAY('chat','stream','structured_input','mcp'),`tool_config`=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','qimen'),`sort_order`=50 WHERE `code`='qimen';
UPDATE `ai_skill` SET `source_type`='skill_repo',`source_ref`='ai-module-skills/Numerologist_skills/ziwei-doushu@ea28c3f',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','birthDate','label','出生日期','type','date','required',TRUE),JSON_OBJECT('key','birthTime','label','出生时间','type','time','required',TRUE),JSON_OBJECT('key','calendarType','label','历法','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','solar','label','公历'),JSON_OBJECT('value','lunar','label','农历'))),JSON_OBJECT('key','gender','label','性别','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','male','label','男'),JSON_OBJECT('value','female','label','女'))),JSON_OBJECT('key','birthplace','label','出生地','type','text','required',TRUE))),`capabilities`=JSON_ARRAY('chat','stream','structured_input','mcp'),`tool_config`=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','ziwei'),`sort_order`=60 WHERE `code`='ziwei';
UPDATE `ai_skill` SET `source_type`='skill_repo',`source_ref`='ai-module-skills/taibu@4ef4d7a',`input_schema`=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','method','label','起卦方式','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','time','label','时间起卦'),JSON_OBJECT('value','numbers','label','数字起卦'),JSON_OBJECT('value','manual','label','手动起卦'))))),`capabilities`=JSON_ARRAY('chat','stream','structured_input','mcp'),`tool_config`=JSON_OBJECT('enabled',FALSE,'server','taibu','tool','liuyao'),`sort_order`=70 WHERE `code`='liuyao';

CREATE TABLE IF NOT EXISTS `ai_session` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `session_no` VARCHAR(32) NOT NULL COMMENT '会话编码',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `skill_code` VARCHAR(32) NOT NULL COMMENT '技能编码',
	`selection_mode` VARCHAR(16) NOT NULL DEFAULT 'explicit',
	`skill_version` VARCHAR(32) NOT NULL DEFAULT '',
  `title` VARCHAR(100) NOT NULL DEFAULT '新对话' COMMENT '会话标题',
  `status` VARCHAR(32) NOT NULL DEFAULT 'active' COMMENT 'active/closed',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_no` (`session_no`),
  KEY `idx_user` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI会话';

INSERT INTO `ai_session` (`session_no`,`user_id`,`skill_code`,`title`,`status`,`create_time`,`update_time`) VALUES
('AI20260630001','1','bazi','今年的事业运势','active','2026-06-30 09:00:00','2026-06-30 09:00:00');

CREATE TABLE IF NOT EXISTS `ai_message` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `session_id` BIGINT NOT NULL COMMENT '会话ID',
  `role` VARCHAR(32) NOT NULL COMMENT 'user/assistant',
  `content` TEXT COMMENT '消息内容',
  `input_json` JSON NULL COMMENT '技能结构化输入快照',
	`attachments_json` JSON NULL COMMENT '图片附件快照',
	`run_id` BIGINT NOT NULL DEFAULT 0,
  `tokens` INT NOT NULL DEFAULT 0 COMMENT 'token消耗',
  `prompt_tokens` INT NOT NULL DEFAULT 0,
  `completion_tokens` INT NOT NULL DEFAULT 0,
  `provider` VARCHAR(32) NOT NULL DEFAULT '',
  `model` VARCHAR(128) NOT NULL DEFAULT '',
  `cost_micros` BIGINT NOT NULL DEFAULT 0,
  `finish_reason` VARCHAR(32) NOT NULL DEFAULT '',
  `status` VARCHAR(16) NOT NULL DEFAULT 'completed' COMMENT 'pending/completed/failed',
	`stage` VARCHAR(32) NOT NULL DEFAULT '',
  `error_message` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Provider失败原因',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '重试次数',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_session` (`session_id`),
  KEY `idx_message_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI消息';

CREATE TABLE IF NOT EXISTS `ai_run` (
	`id` BIGINT NOT NULL AUTO_INCREMENT,
	`run_no` VARCHAR(40) NOT NULL,
	`session_id` BIGINT NOT NULL,
	`message_id` BIGINT NOT NULL,
	`user_id` VARCHAR(64) NOT NULL,
	`skill_code` VARCHAR(32) NOT NULL,
	`skill_version` VARCHAR(32) NOT NULL DEFAULT '',
	`selection_mode` VARCHAR(16) NOT NULL DEFAULT 'explicit',
	`provider` VARCHAR(32) NOT NULL DEFAULT '',
	`model` VARCHAR(128) NOT NULL DEFAULT '',
	`status` VARCHAR(16) NOT NULL DEFAULT 'running',
	`stage` VARCHAR(32) NOT NULL DEFAULT 'accepted',
	`reasoning_tokens` INT NOT NULL DEFAULT 0,
	`error_message` VARCHAR(255) NOT NULL DEFAULT '',
	`started_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	`completed_at` DATETIME(3) NULL,
	PRIMARY KEY (`id`), UNIQUE KEY `uk_run_no` (`run_no`), KEY `idx_message` (`message_id`),
	KEY `idx_user_started` (`user_id`,`started_at`), KEY `idx_session` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI智能体运行记录';

CREATE TABLE IF NOT EXISTS `ai_tool_call` (
	`id` BIGINT NOT NULL AUTO_INCREMENT,
	`run_id` BIGINT NOT NULL,
	`server_code` VARCHAR(32) NOT NULL,
	`tool_name` VARCHAR(64) NOT NULL,
	`arguments_summary` JSON NULL,
	`result_summary` TEXT,
	`status` VARCHAR(16) NOT NULL DEFAULT 'running',
	`latency_ms` INT NOT NULL DEFAULT 0,
	`error_message` VARCHAR(255) NOT NULL DEFAULT '',
	`create_time` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
	`complete_time` DATETIME(3) NULL,
	PRIMARY KEY (`id`), UNIQUE KEY `uk_run_tool` (`run_id`,`tool_name`), KEY `idx_status_time` (`status`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI受控工具调用轨迹';

INSERT INTO `ai_message` (`session_id`,`role`,`content`,`tokens`,`status`,`create_time`) VALUES
(1,'user','请帮我看看今年的事业运势',0,'completed','2026-06-30 09:01:00'),
(1,'assistant','好的，请问您的出生年月日时是？',0,'completed','2026-06-30 09:01:05');

CREATE TABLE IF NOT EXISTS `ai_usage_counter` (
  `user_id` VARCHAR(64) NOT NULL,
  `bucket_type` VARCHAR(16) NOT NULL COMMENT 'minute/day',
  `bucket_start` DATETIME NOT NULL,
  `request_count` INT NOT NULL DEFAULT 0,
  `total_tokens` BIGINT NOT NULL DEFAULT 0,
  `cost_micros` BIGINT NOT NULL DEFAULT 0,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`,`bucket_type`,`bucket_start`),
  KEY `idx_bucket` (`bucket_type`,`bucket_start`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI用户额度计数';

CREATE TABLE IF NOT EXISTS `ai_usage_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `user_id` VARCHAR(64) NOT NULL,
  `session_id` BIGINT NOT NULL,
  `message_id` BIGINT NOT NULL,
  `skill_code` VARCHAR(32) NOT NULL,
  `provider` VARCHAR(32) NOT NULL,
  `model` VARCHAR(128) NOT NULL DEFAULT '',
  `prompt_tokens` INT NOT NULL DEFAULT 0,
  `completion_tokens` INT NOT NULL DEFAULT 0,
  `total_tokens` INT NOT NULL DEFAULT 0,
  `cost_micros` BIGINT NOT NULL DEFAULT 0,
  `status` VARCHAR(16) NOT NULL,
  `latency_ms` INT NOT NULL DEFAULT 0,
  `error_message` VARCHAR(255) NOT NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_message` (`message_id`),
  KEY `idx_user_time` (`user_id`,`create_time`),
  KEY `idx_provider_time` (`provider`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI调用与成本明细';

-- ============================================================
-- 十七、媒体域 askxuan_media（media_asset/live_room）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_media` DEFAULT CHARACTER SET utf8mb4;
INSERT INTO ai_session(session_no,user_id,skill_code,title,status) VALUES('AI-EXPERIENCE-FIXTURE','95101','general','周末想给自己留一点时间','active');
INSERT INTO ai_message(session_id,role,content,status,input_json,attachments_json) VALUES(LAST_INSERT_ID(),'assistant','这是一条隔离验收的历史消息。','completed','{}','[]');
