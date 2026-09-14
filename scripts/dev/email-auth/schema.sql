CREATE DATABASE IF NOT EXISTS askxuan_auth;
CREATE DATABASE IF NOT EXISTS askxuan_user;
USE askxuan_user;
CREATE TABLE `user` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `mobile` VARCHAR(20) NOT NULL COMMENT '手机号',
  `password` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '密码（bcrypt）',
  `nickname` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '昵称',
  `avatar` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '头像',
  `gender` VARCHAR(16) NOT NULL DEFAULT 'unknown' COMMENT '性别',
  `birthday` DATE DEFAULT NULL COMMENT '生日',
  `region` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '所在地',
  `bio` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '简介',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1正常 0禁用',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_mobile` (`mobile`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
USE askxuan_user;
CREATE TABLE IF NOT EXISTS `user_profile` (
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `preference_tags` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '偏好标签，逗号分隔',
  `total_orders` INT NOT NULL DEFAULT 0 COMMENT '累计订单数',
  `total_spent` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '累计消费',
  `last_active_time` DATETIME DEFAULT NULL COMMENT '最后活跃时间',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户画像';
USE askxuan_auth;
CREATE TABLE IF NOT EXISTS `admin_account` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `account` VARCHAR(64) NOT NULL COMMENT '登录账号',
  `password` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '密码（bcrypt）',
  `name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '姓名',
  `role_id` BIGINT NOT NULL DEFAULT 0 COMMENT '角色ID',
  `temple_id` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '所属寺院编码',
  `master_id` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '所属法师编码',
  `shop_id` BIGINT NOT NULL DEFAULT 0 COMMENT '所属商铺ID',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `last_login_time` DATETIME DEFAULT NULL COMMENT '最后登录时间',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_account` (`account`),
  KEY `idx_role` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理台账号';
USE askxuan_auth;
CREATE TABLE IF NOT EXISTS `role` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL COMMENT '角色名称',
  `code` VARCHAR(64) NOT NULL COMMENT '角色编码',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色表';
-- Apply before deploying auth/user services. Existing accounts are never linked by unverified contact data.
CREATE TABLE IF NOT EXISTS askxuan_auth.auth_identity (
 domain VARCHAR(16) NOT NULL,
 user_id BIGINT NOT NULL,
 username VARCHAR(32) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
 email VARCHAR(254) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
 password_hash VARCHAR(255) NOT NULL,
 agreement_version VARCHAR(32) NOT NULL DEFAULT '',
 verified_at DATETIME NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(domain,user_id),
 UNIQUE KEY uk_identity_username(domain,username),
 UNIQUE KEY uk_identity_email(domain,email)
) ENGINE=InnoDB;
ALTER TABLE askxuan_user.user MODIFY mobile VARCHAR(20) NULL;
ALTER TABLE askxuan_user.user MODIFY password VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE askxuan_auth.admin_account MODIFY password VARCHAR(255) NOT NULL;
