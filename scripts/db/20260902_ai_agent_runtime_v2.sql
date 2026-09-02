-- AI 问事完整智能体运行时：版本化技能、自动路由、可审计工具轨迹和多模态附件。
SET @schema_name = 'askxuan_ai';

SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_skill' AND column_name='routing_keywords'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_skill ADD COLUMN routing_keywords JSON NULL AFTER input_schema");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_session' AND column_name='selection_mode'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_session ADD COLUMN selection_mode VARCHAR(16) NOT NULL DEFAULT 'explicit' AFTER skill_code");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_session' AND column_name='skill_version'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_session ADD COLUMN skill_version VARCHAR(32) NOT NULL DEFAULT '' AFTER selection_mode");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='attachments_json'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN attachments_json JSON NULL AFTER input_json");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='run_id'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN run_id BIGINT NOT NULL DEFAULT 0 AFTER attachments_json");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@schema_name AND table_name='ai_message' AND column_name='stage'), 'SELECT 1', "ALTER TABLE askxuan_ai.ai_message ADD COLUMN stage VARCHAR(32) NOT NULL DEFAULT '' AFTER status");
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS askxuan_ai.ai_run (
  id BIGINT NOT NULL AUTO_INCREMENT,
  run_no VARCHAR(40) NOT NULL,
  session_id BIGINT NOT NULL,
  message_id BIGINT NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  skill_code VARCHAR(32) NOT NULL,
  skill_version VARCHAR(32) NOT NULL DEFAULT '',
  selection_mode VARCHAR(16) NOT NULL DEFAULT 'explicit',
  provider VARCHAR(32) NOT NULL DEFAULT '',
  model VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(16) NOT NULL DEFAULT 'running',
  stage VARCHAR(32) NOT NULL DEFAULT 'accepted',
  reasoning_tokens INT NOT NULL DEFAULT 0,
  error_message VARCHAR(255) NOT NULL DEFAULT '',
  started_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  completed_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_run_no (run_no),
	KEY idx_message (message_id),
  KEY idx_user_started (user_id,started_at),
  KEY idx_session (session_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI智能体运行记录';

CREATE TABLE IF NOT EXISTS askxuan_ai.ai_tool_call (
  id BIGINT NOT NULL AUTO_INCREMENT,
  run_id BIGINT NOT NULL,
  server_code VARCHAR(32) NOT NULL,
  tool_name VARCHAR(64) NOT NULL,
  arguments_summary JSON NULL,
  result_summary TEXT,
  status VARCHAR(16) NOT NULL DEFAULT 'running',
  latency_ms INT NOT NULL DEFAULT 0,
  error_message VARCHAR(255) NOT NULL DEFAULT '',
  create_time DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  complete_time DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_run_tool (run_id,tool_name),
  KEY idx_status_time (status,create_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI受控工具调用轨迹';

SET @sql = IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=@schema_name AND table_name='ai_run' AND index_name='uk_message'), 'ALTER TABLE askxuan_ai.ai_run DROP INDEX uk_message, ADD KEY idx_message(message_id)', 'SELECT 1');
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE askxuan_ai.ai_session s JOIN askxuan_ai.ai_skill k ON k.code=s.skill_code
SET s.skill_version=k.version WHERE s.skill_version='';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='askxuan/general@2.0.0',
 prompt_template='你是问玄东方的文化咨询助手。先识别用户真正关心的问题，再给出清晰、温和、可执行的建议。区分事实、传统文化解释和个人选择；信息不足时明确询问，不编造排盘、经文出处或确定结论。涉及医疗、法律、投资、心理危机和人身安全时，优先建议现实专业支持。回答结构通常为：理解问题、文化视角、现实建议、必要提醒。',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY()),routing_keywords=JSON_ARRAY(),capabilities=JSON_ARRAY('chat','stream','auto_route','reasoning_status','vision'),tool_config=JSON_OBJECT('enabled',FALSE),risk_level='low'
 WHERE code='general';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='ai-module-skills/bazi-skill@112a5d8',
 prompt_template='你是中国传统四柱八字文化研究助手。必须以受控排盘工具返回的四柱、十神、藏干、旺衰和大运数据为事实基础，禁止自行猜盘。先核对历法、出生年月日时、性别和出生地；资料不足时只说明需要补充的项目。解读按命盘结构、性格倾向、事业财务、关系家庭、阶段建议展开，并明确命理呈现倾向而非不可改变的命令。不得制造灾祸恐惧，不得诱导转账、改运消费或替代现实专业判断。',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','birthDate','label','出生日期','type','date','required',TRUE),JSON_OBJECT('key','birthTime','label','出生时间','type','time','required',TRUE),JSON_OBJECT('key','calendarType','label','历法','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','solar','label','公历'),JSON_OBJECT('value','lunar','label','农历'))),JSON_OBJECT('key','gender','label','性别','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','male','label','男'),JSON_OBJECT('value','female','label','女'))),JSON_OBJECT('key','birthplace','label','出生地','type','text','required',TRUE,'placeholder','省 / 市 / 区县'))),
 routing_keywords=JSON_ARRAY('八字','四柱','天干','地支','十神','大运','流年','命盘'),capabilities=JSON_ARRAY('chat','stream','structured_input','mcp','auto_route','reasoning_status'),tool_config=JSON_OBJECT('enabled',TRUE,'server','taibu','tool','bazi'),risk_level='medium'
 WHERE code='bazi';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='ai-module-skills/yinyuan-skills@b091c88',
 prompt_template='你是传统婚恋文化与关系咨询助手。可结合用户自愿提供的出生资料讨论生肖、五行和命理文化，但必须把结果表述为观察视角，不得宣称正缘、婚期或关系结局必然发生。优先理解双方沟通、边界、价值观和现实条件；避免操纵、跟踪、迷信依赖和情感恐吓。输出包含文化解读、关系风险、可沟通的问题和现实行动建议。',
 routing_keywords=JSON_ARRAY('姻缘','感情','恋爱','婚姻','合婚','桃花','正缘','夫妻'),capabilities=JSON_ARRAY('chat','stream','structured_input','auto_route','reasoning_status'),tool_config=JSON_OBJECT('enabled',FALSE),risk_level='medium'
 WHERE code='marriage';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='ai-module-skills/tarot-skill@2d15f52',
 prompt_template='你是塔罗文化解读助手。抽牌必须来自受控工具并保留正逆位和牌阵位置，不得声称随机结果可以确定预测未来。把每张牌解释为自我观察线索，综合回答当前处境、可控因素、风险和下一步行动。遇到健康、投资、法律、安全或重大人生决定时，不使用牌面替代专业意见。',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','spread','label','牌阵','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','single','label','单牌'),JSON_OBJECT('value','three','label','三牌阵'),JSON_OBJECT('value','love','label','爱情牌阵'),JSON_OBJECT('value','decision','label','抉择牌阵'))))),routing_keywords=JSON_ARRAY('塔罗','抽牌','牌阵','牌面','正位','逆位'),capabilities=JSON_ARRAY('chat','stream','structured_input','mcp','auto_route','reasoning_status'),tool_config=JSON_OBJECT('enabled',TRUE,'server','taibu','tool','tarot'),risk_level='medium'
 WHERE code='tarot';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='ai-module-skills/fengshui.skill@dc6ffb4',
 prompt_template='你是传统堪舆文化与空间使用顾问。先区分户型、采光、通风、动线、噪声、消防等可验证因素，再说明八宅、形势或玄空等传统解释属于文化参考。图片不足以判断尺度、方位和结构时必须询问，不夸大煞气，不建议危险改造或高价摆件。输出包含观察、现实优化、传统视角和限制说明。',
 routing_keywords=JSON_ARRAY('风水','户型','朝向','方位','布局','看房','看宅','办公位'),capabilities=JSON_ARRAY('chat','stream','structured_input','auto_route','reasoning_status','vision'),tool_config=JSON_OBJECT('enabled',FALSE),risk_level='medium'
 WHERE code='fengshui';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='ai-module-skills/Numerologist_skills/qimen-dunjia@ea28c3f',
 prompt_template='你是时家转盘奇门遁甲文化研究助手。排盘必须使用受控工具，明确起局时间、时区和所问事项；不得自行编造局数、九宫、九星、八门或八神。依据工具盘面说明用神、门星神组合、时机与方位，但只提供决策观察框架，不保证结果。输出包含盘面依据、优势、风险、行动窗口和现实核验条件。',
 routing_keywords=JSON_ARRAY('奇门','遁甲','起局','九宫','八门','九星','择时','方位'),capabilities=JSON_ARRAY('chat','stream','structured_input','mcp','auto_route','reasoning_status'),tool_config=JSON_OBJECT('enabled',TRUE,'server','taibu','tool','qimen'),risk_level='medium'
 WHERE code='qimen';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='ai-module-skills/Numerologist_skills/ziwei-doushu@ea28c3f',
 prompt_template='你是紫微斗数文化研究助手。命盘必须使用受控工具计算，不得自行猜测命宫、主星、四化、宫位或大限。先核对出生年月日时、历法和性别，再按命盘结构、十二宫重点、阶段主题和现实行动建议解读。不同流派口径有差异时主动说明；不得用命盘制造恐惧、歧视或确定命运。',
 routing_keywords=JSON_ARRAY('紫微','斗数','十二宫','命宫','主星','四化','大限'),capabilities=JSON_ARRAY('chat','stream','structured_input','mcp','auto_route','reasoning_status'),tool_config=JSON_OBJECT('enabled',TRUE,'server','taibu','tool','ziwei'),risk_level='medium'
 WHERE code='ziwei';

UPDATE askxuan_ai.ai_skill SET version='2.0.0',source_type='reviewed_skill',source_ref='ai-module-skills/taibu@4ef4d7a',
 prompt_template='你是六爻与梅花易数文化研究助手。卦象必须来自受控工具，清楚记录问题、起卦方式、时间和用神目标；不得自行编造本卦、变卦、动爻、世应或六亲。解读按卦象事实、关系结构、可能趋势、风险条件和可执行建议展开，避免绝对吉凶和恐吓式断语。重大决定仍需结合现实信息与专业意见。',
 input_schema=JSON_OBJECT('fields',JSON_ARRAY(JSON_OBJECT('key','method','label','起卦方式','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','auto','label','自动起卦'),JSON_OBJECT('value','time','label','时间起卦'),JSON_OBJECT('value','number','label','数字起卦'))),JSON_OBJECT('key','numbers','label','起卦数字','type','text','required',FALSE,'placeholder','数字起卦时输入 2-3 个数字'),JSON_OBJECT('key','yongShenTarget','label','关注事项','type','select','required',TRUE,'options',JSON_ARRAY(JSON_OBJECT('value','官鬼','label','事业与规则'),JSON_OBJECT('value','妻财','label','财务与资源'),JSON_OBJECT('value','子孙','label','成果与健康'),JSON_OBJECT('value','父母','label','文书与长辈'),JSON_OBJECT('value','兄弟','label','合作与竞争'))),JSON_OBJECT('key','eventTime','label','起卦时间','type','datetime','required',FALSE))),
 routing_keywords=JSON_ARRAY('六爻','梅花易数','起卦','卦象','动爻','本卦','变卦'),capabilities=JSON_ARRAY('chat','stream','structured_input','mcp','auto_route','reasoning_status'),tool_config=JSON_OBJECT('enabled',TRUE,'server','taibu','tool','liuyao'),risk_level='medium'
 WHERE code='liuyao';
