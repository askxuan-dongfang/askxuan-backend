CREATE DATABASE IF NOT EXISTS askxuan_master;
USE askxuan_master;
CREATE TABLE IF NOT EXISTS `master` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(16) NOT NULL COMMENT '法师编码 M001~M010',
  `dharma_name` VARCHAR(64) NOT NULL COMMENT '法号',
  `lay_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '俗名',
  `temple_code` VARCHAR(16) NOT NULL COMMENT '所属寺院编码',
  `position` VARCHAR(32) NOT NULL COMMENT '职位',
  `belief_code` VARCHAR(32) NOT NULL DEFAULT 'han_buddhism' COMMENT '一级信仰流派编码',
  `sect` VARCHAR(32) NOT NULL COMMENT '宗派',
  `type` VARCHAR(16) NOT NULL COMMENT '类型 佛教/道教',
  `auth_status` VARCHAR(16) NOT NULL COMMENT '认证状态 已认证/待审核',
  `shelf_status` VARCHAR(16) NOT NULL DEFAULT 'off_shelf' COMMENT '上下架状态 on_shelf/off_shelf',
  `platform_status` VARCHAR(16) NOT NULL DEFAULT 'normal' COMMENT '平台状态 normal/banned',
  `manage_by` VARCHAR(16) NOT NULL DEFAULT 'temple' COMMENT '管理方 temple/platform',
  `specialties` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '专长，逗号分隔',
  `avatar` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '头像',
  `rating` DECIMAL(3,2) NOT NULL DEFAULT 0.00,
  `consult_enabled` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否开放即时文字咨询',
  `consult_fee` DECIMAL(10,2) NOT NULL DEFAULT 39.00 COMMENT '单次即时咨询费',
  `consult_valid_hours` INT NOT NULL DEFAULT 72 COMMENT '支付后可发送消息时长',
  `consult_response_minutes` INT NOT NULL DEFAULT 30 COMMENT '承诺首响分钟数',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_temple_code` (`temple_code`),
  KEY `idx_belief_code` (`belief_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师表';
CREATE DATABASE IF NOT EXISTS askxuan_master;
USE askxuan_master;
CREATE TABLE IF NOT EXISTS `master_profile_ext` (
  `master_code` VARCHAR(16) NOT NULL,
  `bio` VARCHAR(512) NOT NULL DEFAULT '',
  `pricing` VARCHAR(512) NOT NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`master_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师工作台资料扩展';
CREATE DATABASE IF NOT EXISTS askxuan_temple;
USE askxuan_temple;
CREATE TABLE IF NOT EXISTS `temple` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `code` VARCHAR(16) NOT NULL COMMENT '寺院编码 T001~T010',
  `name` VARCHAR(64) NOT NULL COMMENT '名称',
  `region` VARCHAR(64) NOT NULL COMMENT '地区',
  `type` VARCHAR(32) NOT NULL COMMENT '类型 汉传佛教/藏传佛教/南传佛教/道教道观/民间地方信仰',
  `belief_code` VARCHAR(32) NOT NULL DEFAULT 'han_buddhism' COMMENT '一级信仰流派编码',
  `sect` VARCHAR(32) NOT NULL COMMENT '宗派 禅宗/全真派/格鲁派/正一派',
  `status` VARCHAR(16) NOT NULL DEFAULT '正常' COMMENT '状态 正常/待审核',
  `address` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '地址',
  `cover_image` VARCHAR(500) NOT NULL DEFAULT '' COMMENT '封面图',
  `rating` DECIMAL(3,2) NOT NULL DEFAULT 0.00 COMMENT '评分',
  `description` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '简介',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_belief_code` (`belief_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院表';
CREATE DATABASE IF NOT EXISTS askxuan_temple;
USE askxuan_temple;
CREATE TABLE IF NOT EXISTS `temple_admin` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `account_id` BIGINT NOT NULL COMMENT '管理台账号ID',
  `role` VARCHAR(32) NOT NULL DEFAULT 'admin' COMMENT 'admin/editor',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_temple_account` (`temple_code`,`account_id`),
  KEY `idx_account` (`account_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院管理员关联';
USE askxuan_auth;
INSERT INTO role(name,code) VALUES ('大师','master'),('寺院管理员','temple_admin'),('平台管理员','platform_super') ON DUPLICATE KEY UPDATE code=VALUES(code);
-- Additive migration: keep all existing accounts and business identifiers.
INSERT INTO askxuan_auth.role(name,code,description) VALUES
('大师认证申请人','master_applicant','仅可管理本人独立大师认证申请'),
('寺院入驻申请人','temple_applicant','仅可管理本人寺院入驻申请')
ON DUPLICATE KEY UPDATE description=VALUES(description);
CREATE TABLE IF NOT EXISTS askxuan_auth.onboarding_application (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 account_id BIGINT NOT NULL UNIQUE,
 kind VARCHAR(16) NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'draft',
 profile_json TEXT NOT NULL,
 revision INT NOT NULL DEFAULT 1,
 review_note VARCHAR(1000) NOT NULL DEFAULT '',
 entity_code VARCHAR(16) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 KEY idx_queue(status,id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_auth.onboarding_event (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 application_id BIGINT NOT NULL,
 actor_id BIGINT NOT NULL,
 action VARCHAR(32) NOT NULL,
 revision INT NOT NULL,
 note VARCHAR(1000) NOT NULL DEFAULT '',
 profile_json TEXT NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY idx_application(application_id,id)
) ENGINE=InnoDB;
-- Private evidence is never exposed through public object storage or listings.
CREATE TABLE IF NOT EXISTS askxuan_auth.onboarding_evidence (
 id VARCHAR(48) PRIMARY KEY,
 account_id BIGINT NOT NULL,
 filename VARCHAR(180) NOT NULL,
 media_type VARCHAR(64) NOT NULL,
 content MEDIUMBLOB NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY idx_owner(account_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_auth.master_invitation (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 account_id BIGINT NOT NULL UNIQUE,
 temple_code VARCHAR(16) NOT NULL,
 master_code VARCHAR(16) NOT NULL UNIQUE,
 email VARCHAR(254) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL UNIQUE,
 username VARCHAR(32) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL UNIQUE,
 status VARCHAR(16) NOT NULL DEFAULT 'pending',
 created_by BIGINT NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;
