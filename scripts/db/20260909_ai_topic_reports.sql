SET NAMES utf8mb4;
-- Report prices are initial configurable test prices; publish after operations review.
CREATE TABLE IF NOT EXISTS askxuan_ai.ai_report_product (
 code VARCHAR(32) PRIMARY KEY, title VARCHAR(80) NOT NULL, subtitle VARCHAR(240) NOT NULL,
 price_cents BIGINT NOT NULL DEFAULT 990, points_price BIGINT NOT NULL DEFAULT 10,
 chapters_json JSON NOT NULL, version VARCHAR(20) NOT NULL DEFAULT '1.0', enabled TINYINT NOT NULL DEFAULT 1
);
CREATE TABLE IF NOT EXISTS askxuan_ai.ai_report (
 id BIGINT PRIMARY KEY AUTO_INCREMENT, report_no VARCHAR(64) NOT NULL UNIQUE,
 user_id VARCHAR(64) NOT NULL, request_key VARCHAR(64) NOT NULL, skill_code VARCHAR(32) NOT NULL,
 title VARCHAR(80) NOT NULL, version VARCHAR(20) NOT NULL, question TEXT NOT NULL, inputs_json JSON NOT NULL,
 chapters_json JSON NOT NULL, price_cents BIGINT NOT NULL, points_price BIGINT NOT NULL,
 status VARCHAR(20) NOT NULL DEFAULT 'generating', summary TEXT NOT NULL, content MEDIUMTEXT NOT NULL,
 chat_session_id BIGINT NOT NULL DEFAULT 0, provider VARCHAR(64) NOT NULL DEFAULT '', model VARCHAR(100) NOT NULL DEFAULT '', prompt_tokens BIGINT NOT NULL DEFAULT 0, completion_tokens BIGINT NOT NULL DEFAULT 0, cost_micros BIGINT NOT NULL DEFAULT 0,
 error_message VARCHAR(240) NOT NULL DEFAULT '', points_paid TINYINT NOT NULL DEFAULT 0,
 created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 UNIQUE KEY user_request(user_id,request_key), KEY user_history(user_id,id)
);
INSERT IGNORE INTO askxuan_ai.ai_report_product(code,title,subtitle,chapters_json) VALUES
('bazi','八字命理','从个人资料出发，理解性格与人生议题',JSON_ARRAY('资料与分析边界','性格与优势','事业与发展','关系与相处','行动与复盘')),
('ziwei','紫微斗数','以传统文化视角，梳理个人发展主题',JSON_ARRAY('资料与分析边界','个人特质','事业主题','关系主题','行动建议')),
('marriage','姻缘合盘','理解彼此差异，找到更好的相处方式',JSON_ARRAY('关系背景','双方需求','沟通与冲突','相处建议','共同计划')),
('fengshui','风水布局','结合空间信息，整理居住与办公建议',JSON_ARRAY('空间信息','使用与动线','光线与环境','布局建议','调整清单')),
('liuyao','六爻占卜','围绕一件具体的事，梳理思路与选择',JSON_ARRAY('问题与背景','分析依据','影响因素','方案比较','行动建议')),
('qimen','奇门遁甲','聚焦当下问题，探索不同选择',JSON_ARRAY('问题与时空资料','分析依据','局势与条件','选择与取舍','行动建议')),
('tarot','塔罗指引','借助象征与提问，探索内心关注',JSON_ARRAY('问题与牌阵','象征解读边界','当前关注','可能的选择','反思与行动'));
