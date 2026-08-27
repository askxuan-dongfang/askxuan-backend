-- 按心愿办收口：只聚合寺院服务与大师服务。
-- 商品映射表仅作历史兼容，不删除旧数据。
SET NAMES utf8mb4;

USE `askxuan_product`;
ALTER TABLE `product_intent_tag` COMMENT='历史商品诉求映射（已停用）';

INSERT INTO `intent_tag`
(`code`,`name`,`description`,`icon`,`landing_type`,`landing_value`,`action_title`,`sort`,`status`) VALUES
('peace','求平安','祈福、护佑与健康相关寺院与大师服务','shield.lefthalf.filled','service','S001','办理平安祈福',10,'enabled'),
('wealth','求财运','财运、供养与事业助力相关寺院与大师服务','banknote.fill','service','S009','办理财运祈福',20,'enabled'),
('love','求姻缘','姻缘、人际与家庭相关寺院与大师服务','heart.fill','service','S008','办理姻缘祈愿',30,'enabled'),
('career','求事业','事业、风水与开光相关寺院与大师服务','briefcase.fill','service','S010','办理事业祈愿',40,'enabled'),
('study','求学业','学业、智慧与考试相关寺院与大师服务','book.fill','service','S013','办理学业祈愿',50,'enabled'),
('taisui','化太岁','本命年与化太岁相关寺院与大师服务','circle.hexagongrid.fill','service','S007','办理化太岁',60,'enabled'),
('diy','定手串','独立手串材料与定制入口，不参与心愿聚合','circle.grid.cross.fill','diy','','开始定制',70,'enabled'),
('rite','做法事','超度等法事相关寺院与大师服务','hands.sparkles.fill','service','S005','预约法事',80,'enabled')
ON DUPLICATE KEY UPDATE
`name`=VALUES(`name`),`description`=VALUES(`description`),`icon`=VALUES(`icon`),
`landing_type`=VALUES(`landing_type`),`landing_value`=VALUES(`landing_value`),
`action_title`=VALUES(`action_title`),`sort`=VALUES(`sort`),`status`=VALUES(`status`);

USE `askxuan_master`;
CREATE TABLE IF NOT EXISTS `master_service_tag` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `master_code` VARCHAR(16) NOT NULL COMMENT '法师编码',
  `service_code` VARCHAR(16) NOT NULL COMMENT '固定服务编码 S001-S013',
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '大师服务价格',
  `status` VARCHAR(16) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled/pending_review',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_master_service` (`master_code`,`service_code`),
  KEY `idx_master_service_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='大师可提供的固定服务';

INSERT INTO `master_service_tag` (`master_code`,`service_code`,`price`,`status`) VALUES
('M001','S001',39.00,'enabled'),('M001','S002',39.00,'enabled'),
('M002','S007',49.00,'enabled'),('M002','S009',49.00,'enabled'),
('M003','S010',39.00,'enabled'),('M003','S013',39.00,'enabled'),
('M004','S001',59.00,'enabled'),('M004','S012',59.00,'enabled'),
('M006','S007',49.00,'enabled'),('M006','S011',49.00,'enabled'),
('M007','S005',39.00,'enabled'),('M008','S001',59.00,'enabled'),
('M009','S012',49.00,'enabled'),('M010','S002',39.00,'enabled')
ON DUPLICATE KEY UPDATE `price`=VALUES(`price`),`status`=VALUES(`status`);

GRANT SELECT ON `askxuan_master`.`master` TO 'product_user'@'%';
GRANT SELECT ON `askxuan_master`.`master_service_tag` TO 'product_user'@'%';
FLUSH PRIVILEGES;
