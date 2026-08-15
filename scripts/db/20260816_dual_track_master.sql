SET NAMES utf8mb4;

-- ============================================================
-- 大师双轨制（寺庙绑定大师 / 野生大师）+ 大师服务标签（幂等迁移）
-- ============================================================

-- 1. master 表新增管理方字段（两库同步：主库 + 大师域克隆库）
USE askxuan;
ALTER TABLE `master`
    ADD COLUMN `manage_by` VARCHAR(16) NOT NULL DEFAULT 'temple' COMMENT '管理方：temple=寺庙绑定 / platform=平台(野生)' AFTER `platform_status`;

USE askxuan_master;
ALTER TABLE `master`
    ADD COLUMN `manage_by` VARCHAR(16) NOT NULL DEFAULT 'temple' COMMENT '管理方：temple=寺庙绑定 / platform=平台(野生)' AFTER `platform_status`;

-- 2. 大师服务标签表（复用 S001-S013 目录）
CREATE TABLE IF NOT EXISTS `master_service_tag` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `master_code` VARCHAR(16) NOT NULL COMMENT '法师编码',
  `service_code` VARCHAR(16) NOT NULL COMMENT '服务编码 S001-S013',
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '大师执行价格（元）',
  `status` VARCHAR(16) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled/pending_review',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_master_service` (`master_code`,`service_code`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='大师服务标签（大师所提供）';

-- 3. 野生大师种子（平台管理，无寺庙；资质已认证）
INSERT INTO `master` (`code`,`dharma_name`,`lay_name`,`temple_code`,`position`,`belief_code`,`sect`,`type`,`auth_status`,`shelf_status`,`platform_status`,`manage_by`,`specialties`,`avatar`,`rating`,`consult_enabled`,`consult_fee`,`consult_valid_hours`,`consult_response_minutes`) VALUES
('W001','云游道人（演示）','陈道玄','','独立道长','taoism','正一派','道教','已认证','on_shelf','normal','platform','八字命理,风水堪舆','',4.9,1,39.00,72,30),
('W002','慧远居士（演示）','周明远','','独立居士','han_buddhism','净土宗','汉传佛教','已认证','on_shelf','normal','platform','姻缘测算,六爻梅花','',4.8,1,29.00,72,30),
('W003','紫微先生（演示）','吴子谦','','命理师','taoism','全真派','道教','已认证','on_shelf','normal','platform','紫微斗数,奇门遁甲','',4.9,1,49.00,72,30),
('W004','塔罗心语（演示）','林小满','','占卜师','han_buddhism','禅宗','汉传佛教','已认证','on_shelf','normal','platform','塔罗牌,姻缘测算','',4.7,1,19.00,72,30)
ON DUPLICATE KEY UPDATE `manage_by`='platform', `temple_code`='';

-- 4. 野生大师法师端账号（平台分发；角色 role_id=3 即 master）
INSERT INTO askxuan_auth.admin_account (`account`,`password`,`name`,`role_id`,`temple_id`,`master_id`,`status`) VALUES
('w001','123456','云游道人（演示）',3,'','W001','enabled'),
('w002','123456','慧远居士（演示）',3,'','W002','enabled'),
('w003','123456','紫微先生（演示）',3,'','W003','enabled'),
('w004','123456','塔罗心语（演示）',3,'','W004','enabled')
ON DUPLICATE KEY UPDATE `status`='enabled';

-- 5. 资质认证记录（平台已认证）
INSERT INTO `master_credential` (`master_code`,`cert_type`,`cert_url`,`status`,`submit_time`,`audit_time`) VALUES
('W001','identity','','已认证',NOW(),NOW()),
('W002','identity','','已认证',NOW(),NOW()),
('W003','identity','','已认证',NOW(),NOW()),
('W004','identity','','已认证',NOW(),NOW())
ON DUPLICATE KEY UPDATE `status`='已认证';

-- 6. 野生大师服务标签
INSERT INTO `master_service_tag` (`master_code`,`service_code`,`price`,`status`) VALUES
('W001','S001',99.00,'enabled'),
('W001','S007',88.00,'enabled'),
('W002','S006',66.00,'enabled'),
('W002','S008',58.00,'enabled'),
('W003','S004',128.00,'enabled'),
('W003','S010',108.00,'enabled'),
('W004','S002',36.00,'enabled'),
('W004','S009',46.00,'enabled')
ON DUPLICATE KEY UPDATE `status`='enabled';
