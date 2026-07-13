-- ============================================================
-- 问玄东方 数据库初始化脚本
-- 依据《统一数据字典.md》建表与种子数据
-- 数据库：askxuan (MySQL 8.0)
-- 字符集：utf8mb4
-- 执行方式：docker exec -i askxuan-mysql mysql -uroot -proot123 askxuan < askXuan-backend/db/init.sql
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- 1. 寺院表 temple（数据字典第1节，6 寺院）
-- ----------------------------
DROP TABLE IF EXISTS `temple`;
CREATE TABLE `temple` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '自增主键',
  `code` VARCHAR(16) NOT NULL COMMENT '寺院编码 T001~T006',
  `name` VARCHAR(64) NOT NULL COMMENT '名称',
  `region` VARCHAR(64) NOT NULL COMMENT '地区',
  `type` VARCHAR(32) NOT NULL COMMENT '类型 汉传佛教/藏传佛教/南传佛教/道教道观/民间地方信仰',
  `belief_code` VARCHAR(32) NOT NULL DEFAULT 'han_buddhism' COMMENT '一级信仰流派编码',
  `sect` VARCHAR(32) NOT NULL COMMENT '宗派 禅宗/全真派/格鲁派/正一派',
  `status` VARCHAR(16) NOT NULL DEFAULT '正常' COMMENT '状态 正常/待审核',
  `address` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '地址',
  `cover_image` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '封面图',
  `rating` DECIMAL(3,2) NOT NULL DEFAULT 0.00 COMMENT '评分',
  `description` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '简介',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_belief_code` (`belief_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院表';

INSERT INTO `temple` (`code`,`name`,`region`,`type`,`belief_code`,`sect`,`status`,`address`,`cover_image`,`rating`,`description`) VALUES
('T001','灵隐寺','浙江杭州','汉传佛教','han_buddhism','禅宗','正常','杭州市西湖区灵隐路法云弄1号','/assets/temple-card-lingyinsi.jpg',4.90,'杭州最早的名刹，江南禅宗五大名山之一，以禅修、祈福、开光法事闻名。'),
('T002','白云观','北京','道教','daoism','全真派','正常','北京市西城区白云观街1号','/assets/temple-card-baimasi.jpg',4.70,'道教全真派三大祖庭之一，北京最大道观，以道教科仪、祈福、化太岁闻名。'),
('T003','少林寺','河南嵩山','汉传佛教','han_buddhism','禅宗','正常','河南省郑州市登封市嵩山少林景区','/assets/temple-card-shaolinsi.jpg',4.80,'禅宗祖庭，少林武术发源地，以禅修、武术、超度、开光法事闻名。'),
('T004','大昭寺','西藏拉萨','藏传佛教','tibetan_buddhism','格鲁派','正常','拉萨市城关区八廓街','/assets/temple-card-dazhaosi.jpg',4.90,'藏传佛教圣地，拉萨城市中心，以藏密仪轨、灌顶、超度、祈福闻名。'),
('T005','普陀山','浙江舟山','汉传佛教','han_buddhism','禅宗','待审核','舟山市普陀区普陀山','/assets/temple-card-famensi.jpg',4.60,'观音菩萨道场，佛教四大名山之一，以净土法门、观音法门、祈福闻名。'),
('T006','武当山','湖北十堰','道教','daoism','正一派','正常','十堰市丹江口市武当山特区','/assets/temple-card-qingyanggong.jpg',4.70,'道教圣地，真武大帝道场，以内丹、太极、风水、化太岁闻名。');

DROP TABLE IF EXISTS `belief_profile`;
CREATE TABLE `belief_profile` (
  `code` VARCHAR(32) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `summary` VARCHAR(255) NOT NULL DEFAULT '',
  `description` TEXT NOT NULL,
  `cover_image` VARCHAR(500) NOT NULL DEFAULT '',
  `sort` INT NOT NULL DEFAULT 0,
  `status` VARCHAR(16) NOT NULL DEFAULT 'enabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='一级信仰流派运营资料';

INSERT INTO `belief_profile` (`code`,`name`,`summary`,`description`,`sort`) VALUES
('han_buddhism','汉传佛教','慈悲与智慧并行','汉传佛教在中国长期发展，形成禅、净土、天台、华严等具体宗派。平台以一级流派聚合寺院和法师，同时保留具体宗派信息。',10),
('tibetan_buddhism','藏传佛教','传承、修持与慈悲','藏传佛教具有清晰的传承体系，包含格鲁、宁玛、噶举、萨迦等具体宗派。',20),
('daoism','道教','道法自然，清静修持','道教是中国本土宗教传统，平台一级归类为道教，并保留全真、正一等具体宗派。',30),
('folk','民间信仰','乡土传统与民俗传承','民间信仰承载地域性祭祀、祈愿和文化传统，相关内容须遵循平台审核与合规要求。',40);

-- ----------------------------
-- 2. 法师表 master（数据字典第2节，6 法师）
-- ----------------------------
DROP TABLE IF EXISTS `master`;
CREATE TABLE `master` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(16) NOT NULL COMMENT '法师编码 M001~M006',
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
  `specialties` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '专长，逗号分隔',
  `avatar` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '头像',
  `rating` DECIMAL(3,2) NOT NULL DEFAULT 0.00,
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_temple_code` (`temple_code`),
  KEY `idx_belief_code` (`belief_code`),
  CONSTRAINT `fk_master_temple` FOREIGN KEY (`temple_code`) REFERENCES `temple` (`code`)
    ON UPDATE CASCADE ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师表';

INSERT INTO `master` (`code`,`dharma_name`,`lay_name`,`temple_code`,`position`,`belief_code`,`sect`,`type`,`auth_status`,`shelf_status`,`platform_status`,`specialties`,`avatar`,`rating`) VALUES
('M001','智海法师','陈建华','T001','住持','han_buddhism','禅宗','佛教','已认证','on_shelf','normal','佛学,禅修,开光,祈福','/assets/master-avatar-zhihai.jpg',4.90),
('M002','清风道长','李信军','T002','监院','daoism','全真道派','道教','已认证','on_shelf','normal','道学,风水,命理,祈福','/assets/master-avatar-qingfeng.jpg',4.80),
('M003','释延心法师','王建军','T003','首座','han_buddhism','禅宗','佛教','已认证','on_shelf','normal','武术,禅修,超度,开光','/assets/master-avatar-zhihai.jpg',4.80),
('M004','扎西多吉活佛','—','T004','活佛','tibetan_buddhism','藏密佛教','佛教','已认证','on_shelf','normal','藏密仪轨,灌顶,超度,祈福','/assets/master-avatar-zhaxiduoji.jpg',5.00),
('M005','慧明法师','周明华','T005','副住持','han_buddhism','净土宗','佛教','待审核','off_shelf','normal','净土,观音法门,祈福','/assets/master-avatar-shimingyuan.jpg',4.50),
('M006','真武道长','张志远','T006','知客','daoism','正一派','道教','已认证','on_shelf','normal','内丹,太极,风水,化太岁','/assets/master-avatar-zhangzhishun.jpg',4.70);

-- ----------------------------
-- 3. 服务类型表 service_type（数据字典第3节，用户端服务）
-- ----------------------------
DROP TABLE IF EXISTS `service_type`;
CREATE TABLE `service_type` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(16) NOT NULL COMMENT '服务编码 S001~S013',
  `name` VARCHAR(64) NOT NULL COMMENT '服务名称',
  `type` VARCHAR(32) NOT NULL COMMENT '类型 法事/供养',
  `price_range` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '默认价格区间',
  `master_codes` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '关联法师编码，逗号分隔',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务类型表';

INSERT INTO `service_type` (`code`,`name`,`type`,`price_range`,`master_codes`) VALUES
('S001','祈福','法事','¥100-500','M001,M002,M004,M005'),
('S002','供灯','供养','¥50-300','M001,M003,M004'),
('S003','上香','供养','¥30-200','M001,M003,M004'),
('S004','还愿','法事','¥100-500','M001,M003,M004'),
('S005','超度','法事','¥300-1000','M001,M003,M004'),
('S006','开光','法事','¥168-500','M001,M003'),
('S007','化太岁','法事','¥200-800','M002,M006'),
('S008','求姻缘','祈愿','¥100-500','M001,M005'),
('S009','求财运','祈愿','¥100-800','M002,M006'),
('S010','求事业','祈愿','¥100-600','M003,M006'),
('S011','求风水','咨询','¥200-1000','M002,M006'),
('S012','求健康','祈愿','¥100-600','M001,M004'),
('S013','求学业','祈愿','¥100-500','M003,M005');

-- ----------------------------
-- 4. 加持服务表 extra_service（数据字典第4节，4 项加持，价格精确匹配）
-- ----------------------------
DROP TABLE IF EXISTS `extra_service`;
CREATE TABLE `extra_service` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(16) NOT NULL COMMENT '加持服务编码 E001~E004',
  `name` VARCHAR(64) NOT NULL COMMENT '服务名称',
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `master_code` VARCHAR(16) NOT NULL COMMENT '法师编码',
  `price` DECIMAL(10,2) NOT NULL COMMENT '价格（精确匹配）',
  `description` VARCHAR(512) NOT NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_extra_temple` (`temple_code`),
  KEY `idx_extra_master` (`master_code`),
  CONSTRAINT `fk_extra_service_temple` FOREIGN KEY (`temple_code`) REFERENCES `temple` (`code`)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `fk_extra_service_master` FOREIGN KEY (`master_code`) REFERENCES `master` (`code`)
    ON UPDATE CASCADE ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='DIY加持服务表';

INSERT INTO `extra_service` (`code`,`name`,`temple_code`,`master_code`,`price`,`description`) VALUES
('E001','灵隐寺·开光加持','T001','M001',168.00,'智海法师亲自为手串开光加持，注入禅宗正能量'),
('E002','白云观·道长祈福','T002','M002',128.00,'清风道长以全真派科仪为手串祈福消灾'),
('E003','少林寺·武僧加持','T003','M003',198.00,'释延心法师以少林禅功为手串加持护身'),
('E004','大昭寺·活佛灌顶','T004','M004',268.00,'扎西多吉活佛以藏密仪轨为手串灌顶加持');

-- ----------------------------
-- 5. 材料表 material（数据字典第5节，14 材料）
-- ----------------------------
DROP TABLE IF EXISTS `material`;
CREATE TABLE `material` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL COMMENT '材料名称',
  `spec` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '规格',
  `unit_price` DECIMAL(10,2) NOT NULL COMMENT '单价',
  `unit` VARCHAR(16) NOT NULL DEFAULT '颗' COMMENT '单位',
  `category` VARCHAR(32) NOT NULL DEFAULT 'main_bead' COMMENT '材料分类',
  `five_elements` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '五行属性',
  `image` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '材料图片',
  `stock` INT NOT NULL DEFAULT 0 COMMENT '库存',
  `status` VARCHAR(32) NOT NULL DEFAULT 'on_shelf' COMMENT '状态：on_shelf/off_shelf',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name_spec` (`name`,`spec`),
  KEY `idx_category_status` (`category`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='DIY手串材料表';

INSERT INTO `material` (`name`,`spec`,`unit_price`,`unit`,`category`,`five_elements`,`image`,`stock`,`status`) VALUES
('小叶紫檀圆珠','10mm',28.00,'颗','main_bead','wood','/assets/materials/rosewood.jpg',500,'on_shelf'),
('星月菩提','10mm',18.00,'颗','main_bead','wood','/assets/materials/bodhi.jpg',500,'on_shelf'),
('凤眼菩提','10mm',22.00,'颗','main_bead','wood','/assets/materials/rudraksha.jpg',500,'on_shelf'),
('白玉','8mm',35.00,'颗','main_bead','earth','/assets/materials/jade.jpg',300,'on_shelf'),
('青金石','10mm',25.00,'颗','main_bead','water','/assets/materials/lapis.jpg',300,'on_shelf'),
('南红玛瑙','8mm',32.00,'颗','main_bead','fire','/assets/materials/agate.jpg',300,'on_shelf'),
('蜜蜡','10mm',45.00,'颗','main_bead','earth','/assets/materials/amber.jpg',260,'on_shelf'),
('黑曜石','10mm',12.00,'颗','main_bead','water','/assets/materials/obsidian.jpg',500,'on_shelf'),
('藏银三通','10mm',48.00,'个','three_way','metal','/assets/materials/silver-three-way.jpg',120,'on_shelf'),
('蜜蜡佛头','12mm',68.00,'个','buddha_head','earth','/assets/materials/amber-head.jpg',120,'on_shelf'),
('花丝莲花吊坠','15mm',20.00,'个','pendant','metal','/assets/materials/lotus-pendant.jpg',200,'on_shelf'),
('白水晶隔片','6mm',2.50,'颗','spacer','water','/assets/materials/crystal-spacer.jpg',1000,'on_shelf'),
('流苏配饰','',28.00,'个','tassel','fire','/assets/materials/tassel.jpg',180,'on_shelf'),
('弹力绳','',2.00,'根','cord','wood','/assets/materials/cord.jpg',1000,'on_shelf');

-- ----------------------------
-- 6. 用户表 user
-- ----------------------------
DROP TABLE IF EXISTS `user`;
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

-- mock 用户（密码明文 123456，联调阶段使用；生产环境应存 bcrypt）
INSERT INTO `user` (`mobile`,`password`,`nickname`,`avatar`,`gender`,`region`,`bio`) VALUES
('13800138000','123456','善信居士','/assets/master-avatar-zhihai.jpg','male','浙江杭州','心向菩提，常诵心经。'),
('13800138001','123456','智海法师','/assets/master-avatar-zhihai.jpg','male','浙江杭州','灵隐寺住持。'),
('13800138002','123456','平台管理员','/assets/master-avatar-qingfeng.jpg','unknown','北京','');

-- ----------------------------
-- 7. 预约订单表 booking（数据字典第7节状态流转）
-- ----------------------------
DROP TABLE IF EXISTS `booking`;
CREATE TABLE `booking` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `booking_no` VARCHAR(32) NOT NULL COMMENT '预约单号 B20260630001',
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `master_code` VARCHAR(16) NOT NULL COMMENT '法师编码',
  `service_code` VARCHAR(16) NOT NULL COMMENT '服务编码',
  `booking_date` DATE NOT NULL COMMENT '预约日期',
  `time_slot` VARCHAR(32) NOT NULL COMMENT '时段',
  `merit_money` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '功德金',
  `merit_money_tier` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '功德金档位',
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/confirmed/in_progress/completed/cancelled/reviewed',
  `note` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '备注',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_booking_no` (`booking_no`),
  KEY `idx_user` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='预约订单表';

INSERT INTO `booking` (`booking_no`,`user_id`,`temple_code`,`master_code`,`service_code`,`booking_date`,`time_slot`,`merit_money`,`merit_money_tier`,`status`,`note`,`create_time`) VALUES
('B20260630001',1001,'T001','M001','S001','2026-07-05','09:00-10:00',200.00,'大额','pending','为家人祈求平安健康。','2026-06-30 08:30:00'),
('B20260628002',1002,'T003','M003','S005','2026-07-02','14:00-15:30',500.00,'不限额','confirmed','为先人超度往生，请法师主持法事。','2026-06-28 16:20:00'),
('B20260615003',1001,'T002','M002','S007','2026-06-20','10:00-11:00',100.00,'中额','completed','本命年化太岁，祈求流年顺利。','2026-06-15 19:45:00');

-- ----------------------------
-- 8. 站内消息表 message
-- ----------------------------
DROP TABLE IF EXISTS `message`;
CREATE TABLE `message` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `user_id` VARCHAR(32) NOT NULL COMMENT '接收用户ID',
  `title` VARCHAR(128) NOT NULL COMMENT '标题',
  `content` VARCHAR(512) NOT NULL COMMENT '内容',
  `biz_type` VARCHAR(32) NOT NULL DEFAULT 'booking' COMMENT '业务类型',
  `biz_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '关联业务ID',
  `is_read` TINYINT NOT NULL DEFAULT 0 COMMENT '0未读 1已读',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_read` (`user_id`,`is_read`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站内消息表';

INSERT INTO `message` (`user_id`,`title`,`content`,`biz_type`,`biz_id`,`is_read`,`create_time`) VALUES
('1001','预约已创建','您的预约（灵隐寺·祈福）已提交，请等待寺院确认。','booking','B20260630001',0,'2026-06-30 08:30:05'),
('1002','预约已确认','您的预约（少林寺·超度）已被寺院确认，请按时到达。','booking','B20260628002',0,'2026-06-28 17:00:00'),
('1001','预约已完成','您的预约（白云观·化太岁）已完成，感谢您的功德。','booking','B20260615003',1,'2026-06-20 11:30:00');

-- ----------------------------
-- 9. 功德金档位表 merit_money_tier（数据字典第6节）
-- ----------------------------
DROP TABLE IF EXISTS `merit_money_tier`;
CREATE TABLE `merit_money_tier` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `tier` VARCHAR(32) NOT NULL COMMENT '档位名称',
  `amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '金额（-1 表示不限额）',
  `description` VARCHAR(128) NOT NULL DEFAULT '',
  `sort` INT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tier` (`tier`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='功德金档位表';

INSERT INTO `merit_money_tier` (`tier`,`amount`,`description`,`sort`) VALUES
('随喜',0.00,'随喜功德，不限金额',1),
('小额',50.00,'小额功德金',2),
('中额',100.00,'中额功德金',3),
('大额',200.00,'大额功德金',4),
('不限额',-1.00,'自定义输入金额',5);

-- ============================================================
-- 以下为各业务域分库表（依据 backend-overview.md §4 分库策略）
-- 现有 9 张 MVP-1 核心表保留在默认 askxuan 库；新增表按域独立建库。
-- 字段定义来源：各服务 internal/model/*.go Go 结构体
-- ============================================================

-- ============================================================
-- 一、认证域 askxuan_auth（admin_account/role/permission/role_permission）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_auth` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_auth`;

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

INSERT INTO `admin_account` (`account`,`password`,`name`,`role_id`,`temple_id`,`master_id`,`status`,`create_time`) VALUES
('admin','123456','平台超管',1,'','','enabled','2026-07-01 00:00:00'),
('lingyin_admin','123456','灵隐寺管理员',2,'T001','','enabled','2026-07-01 00:00:00'),
('zhihai','123456','智海法师',3,'T001','M001','enabled','2026-07-01 00:00:00');

CREATE TABLE IF NOT EXISTS `role` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL COMMENT '角色名称',
  `code` VARCHAR(64) NOT NULL COMMENT '角色编码',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色表';

INSERT INTO `role` (`id`,`name`,`code`,`description`,`create_time`) VALUES
(1,'平台超管','platform_super','全平台运营管理','2026-07-01 00:00:00'),
(2,'寺院管理员','temple_admin','管理本寺院信息/法师/服务/预约','2026-07-01 00:00:00'),
(3,'法师','master','管理个人日程/预约/加持任务','2026-07-01 00:00:00'),
(4,'商城运营','shop_admin','管理商品/DIY订单/材料库','2026-07-01 00:00:00'),
(5,'平台客服','platform_service','处理投诉/咨询/举报','2026-07-01 00:00:00');

CREATE TABLE IF NOT EXISTS `permission` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(64) NOT NULL COMMENT '权限编码',
  `name` VARCHAR(64) NOT NULL COMMENT '权限名称',
  `resource` VARCHAR(64) NOT NULL COMMENT '资源',
  `action` VARCHAR(32) NOT NULL COMMENT '动作',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='权限表';

INSERT INTO `permission` (`id`,`code`,`name`,`resource`,`action`) VALUES
(1,'temple:read','查看寺院','temple','read'),
(2,'temple:update','编辑寺院','temple','update'),
(3,'master:read','查看法师','master','read'),
(4,'master:create','添加法师','master','create'),
(5,'booking:read','查看预约','booking','read'),
(6,'booking:confirm','确认预约','booking','update'),
(7,'blessing:assign','分配加持任务','blessing','update'),
(8,'blessing:process','处理加持任务','blessing','update'),
(9,'user:ban','封禁用户','user','update'),
(10,'audit:review','审核操作','audit','update');

CREATE TABLE IF NOT EXISTS `role_permission` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `role_id` BIGINT NOT NULL,
  `permission_id` BIGINT NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_perm` (`role_id`,`permission_id`),
  KEY `idx_permission` (`permission_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色权限关联表';

INSERT INTO `role_permission` (`role_id`,`permission_id`) VALUES
(1,1),(1,2),(1,3),(1,4),(1,5),(1,6),(1,7),(1,8),(1,9),(1,10),
(2,1),(2,2),(2,3),(2,4),(2,5),(2,6),(2,7),
(3,8);

-- ============================================================
-- 二、用户域 askxuan_user（user_address/user_profile，user 表保留在 askxuan 库）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_user` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_user`;

CREATE TABLE IF NOT EXISTS `user_address` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `name` VARCHAR(64) NOT NULL COMMENT '收货人',
  `phone` VARCHAR(20) NOT NULL COMMENT '手机号',
  `province` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '省',
  `city` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '市',
  `district` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '区',
  `detail` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '详细地址',
  `is_default` TINYINT NOT NULL DEFAULT 0 COMMENT '0否 1是',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户收货地址';

INSERT INTO `user_address` (`user_id`,`name`,`phone`,`province`,`city`,`district`,`detail`,`is_default`,`create_time`) VALUES
(1001,'善信居士','13800138000','浙江省','杭州市','西湖区','灵隐路法云弄1号',1,'2026-06-30 10:00:00');

CREATE TABLE IF NOT EXISTS `user_profile` (
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `preference_tags` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '偏好标签，逗号分隔',
  `total_orders` INT NOT NULL DEFAULT 0 COMMENT '累计订单数',
  `total_spent` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '累计消费',
  `last_active_time` DATETIME DEFAULT NULL COMMENT '最后活跃时间',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户画像';

INSERT INTO `user_profile` (`user_id`,`preference_tags`,`total_orders`,`total_spent`,`last_active_time`) VALUES
(1001,'祈福,开光',3,600.00,'2026-06-30 18:00:00');

-- ============================================================
-- 三、寺院域 askxuan_temple（temple_image/temple_admin/temple_audit/temple_service/service_schedule）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_temple` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_temple`;

CREATE TABLE IF NOT EXISTS `temple_image` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `url` VARCHAR(255) NOT NULL COMMENT '图片URL',
  `type` VARCHAR(32) NOT NULL DEFAULT 'detail' COMMENT 'cover/detail/hero',
  `sort` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_temple` (`temple_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院图片';

INSERT INTO `temple_image` (`temple_code`,`url`,`type`,`sort`,`create_time`) VALUES
('T001','/assets/temple-card-lingyinsi.jpg','cover',0,'2026-06-01 10:00:00'),
('T001','/assets/temple-detail-lingyinsi-1.jpg','detail',1,'2026-06-01 10:00:00'),
('T002','/assets/temple-card-baimasi.jpg','cover',0,'2026-06-01 10:00:00');

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

INSERT INTO `temple_admin` (`temple_code`,`account_id`,`role`,`create_time`) VALUES
('T001',1001,'admin','2026-07-01 00:00:00'),
('T002',1002,'admin','2026-07-01 00:00:00');

CREATE TABLE IF NOT EXISTS `temple_audit` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `applicant_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '申请人名称',
  `contact_phone` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '联系电话',
  `cert_urls` TEXT COMMENT '资质图片URL，JSON数组',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/first_pass/final_pass/rejected',
  `auditor_id` BIGINT NOT NULL DEFAULT 0 COMMENT '审核人ID',
  `audit_remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '审核备注',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`),
  KEY `idx_temple` (`temple_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院入驻审核';

INSERT INTO `temple_audit` (`temple_code`,`applicant_name`,`contact_phone`,`cert_urls`,`status`,`create_time`) VALUES
('T005','普陀山管委会','0580-1234567','["/assets/cert-putuoshan-1.jpg"]','pending','2026-06-28 09:00:00');

CREATE TABLE IF NOT EXISTS `temple_service` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `service_code` VARCHAR(16) NOT NULL COMMENT '服务类型编码',
  `service_name` VARCHAR(64) NOT NULL COMMENT '服务名称',
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '寺院定价',
  `time_slots` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '时段，JSON数组',
  `status` VARCHAR(32) NOT NULL DEFAULT 'on_shelf' COMMENT 'on_shelf/off_shelf',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_temple` (`temple_code`),
  KEY `idx_service_code` (`service_code`),
  KEY `idx_status` (`status`),
  UNIQUE KEY `uk_temple_service` (`temple_code`,`service_code`),
  CONSTRAINT `fk_temple_service_temple` FOREIGN KEY (`temple_code`) REFERENCES `temple` (`code`)
    ON UPDATE CASCADE ON DELETE RESTRICT,
  CONSTRAINT `fk_temple_service_type` FOREIGN KEY (`service_code`) REFERENCES `service_type` (`code`)
    ON UPDATE CASCADE ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院自定义服务定价';

CREATE TABLE IF NOT EXISTS `temple_service_intent_tag` (
  `temple_service_id` BIGINT NOT NULL COMMENT '寺院服务ID',
  `tag_code` VARCHAR(32) NOT NULL COMMENT '诉求标签编码',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`temple_service_id`,`tag_code`),
  KEY `idx_intent_tag_code` (`tag_code`),
  CONSTRAINT `fk_intent_temple_service` FOREIGN KEY (`temple_service_id`) REFERENCES `temple_service` (`id`)
    ON UPDATE CASCADE ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院服务诉求标签映射';

INSERT INTO `temple_service` (`temple_code`,`service_code`,`service_name`,`price`,`time_slots`,`status`,`create_time`) VALUES
('T001','S001','祈福',200.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T001','S002','供灯',80.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T001','S006','开光',500.00,'["10:00-11:00"]','on_shelf','2026-06-01 10:00:00'),
('T001','S008','求姻缘',260.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T001','S012','求健康',260.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T002','S001','祈福',128.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T002','S003','上香',60.00,'["08:00-11:00","14:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T002','S007','化太岁',300.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T002','S009','求财运',300.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T002','S011','求风水',688.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T003','S001','祈福',200.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T003','S005','超度',500.00,'["14:00-15:30"]','on_shelf','2026-06-01 10:00:00'),
('T003','S006','开光',360.00,'["10:00-11:00"]','on_shelf','2026-06-01 10:00:00'),
('T003','S010','求事业',280.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T003','S013','求学业',220.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T004','S001','祈福',268.00,'["10:00-12:00","15:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T004','S002','供灯',120.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T004','S005','超度',600.00,'["14:00-16:00"]','on_shelf','2026-06-01 10:00:00'),
('T004','S012','求健康',360.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T005','S001','祈福',180.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T005','S002','供灯',80.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T005','S008','求姻缘',260.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T005','S013','求学业',220.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T006','S001','祈福',168.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T006','S003','上香',66.00,'["08:00-11:00","14:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T006','S007','化太岁',388.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T006','S010','求事业',280.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00'),
('T006','S011','求风水',688.00,'["09:00-12:00","13:00-17:00"]','on_shelf','2026-06-01 10:00:00');

INSERT IGNORE INTO `temple_service_intent_tag` (`temple_service_id`,`tag_code`)
SELECT id, CASE service_code
  WHEN 'S001' THEN 'peace' WHEN 'S002' THEN 'love' WHEN 'S003' THEN 'wealth'
  WHEN 'S005' THEN 'rite' WHEN 'S006' THEN 'career' WHEN 'S007' THEN 'taisui'
  WHEN 'S008' THEN 'love' WHEN 'S009' THEN 'wealth' WHEN 'S010' THEN 'career'
  WHEN 'S011' THEN 'career' WHEN 'S012' THEN 'peace' WHEN 'S013' THEN 'study'
END FROM `temple_service` WHERE service_code IN ('S001','S002','S003','S005','S006','S007','S008','S009','S010','S011','S012','S013');

CREATE TABLE IF NOT EXISTS `service_schedule` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `service_id` BIGINT NOT NULL COMMENT '寺院服务ID（temple_service.id）',
  `schedule_date` DATE NOT NULL COMMENT '日期',
  `time_slot` VARCHAR(32) NOT NULL COMMENT '时段',
  `capacity` INT NOT NULL DEFAULT 1 COMMENT '可预约容量',
  `booked_count` INT NOT NULL DEFAULT 0 COMMENT '已预约数',
  `status` VARCHAR(32) NOT NULL DEFAULT 'available' COMMENT 'available/booked/off',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_temple_date` (`temple_code`,`schedule_date`),
  KEY `idx_service` (`service_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='服务时段';

-- ============================================================
-- 四、法师域 askxuan_master（master_credential/master_schedule/master_audit）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_master` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_master`;

CREATE TABLE IF NOT EXISTS `master_credential` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `master_code` VARCHAR(16) NOT NULL COMMENT '法师编码',
  `cert_type` VARCHAR(32) NOT NULL COMMENT '证书类型 ordination/identity/title',
  `cert_url` VARCHAR(255) NOT NULL COMMENT '证书图片URL',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/verified/rejected',
  `submit_time` DATETIME DEFAULT NULL COMMENT '提交时间',
  `audit_time` DATETIME DEFAULT NULL COMMENT '审核时间',
  PRIMARY KEY (`id`),
  KEY `idx_master` (`master_code`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师资质证书';

INSERT INTO `master_credential` (`master_code`,`cert_type`,`cert_url`,`status`,`submit_time`,`audit_time`) VALUES
('M001','ordination','/assets/cred-m001-1.jpg','verified','2026-06-01 10:00:00','2026-06-02 14:00:00'),
('M002','ordination','/assets/cred-m002-1.jpg','verified','2026-06-01 10:00:00','2026-06-02 14:00:00'),
('M005','ordination','/assets/cred-m005-1.jpg','pending','2026-06-28 09:00:00',NULL);

CREATE TABLE IF NOT EXISTS `master_schedule` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `master_code` VARCHAR(16) NOT NULL COMMENT '法师编码',
  `schedule_date` DATE NOT NULL COMMENT '日期',
  `time_slots` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '时段，JSON数组',
  `status` VARCHAR(32) NOT NULL DEFAULT 'available' COMMENT 'available/booked/off',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_master_date` (`master_code`,`schedule_date`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师排班';

INSERT INTO `master_schedule` (`master_code`,`schedule_date`,`time_slots`,`status`,`create_time`) VALUES
('M001','2026-07-05','["09:00-12:00","13:00-17:00"]','available','2026-06-30 10:00:00');

CREATE TABLE IF NOT EXISTS `master_audit` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `master_code` VARCHAR(16) NOT NULL COMMENT '法师编码',
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `credential_urls` TEXT COMMENT '资质图片URL，JSON数组',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/pass/rejected',
  `auditor_id` BIGINT NOT NULL DEFAULT 0 COMMENT '审核人ID',
  `audit_remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '审核备注',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`),
  KEY `idx_master` (`master_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师资质审核';

INSERT INTO `master_audit` (`master_code`,`temple_code`,`credential_urls`,`status`,`create_time`) VALUES
('M005','T005','["/assets/master-cert-m005-1.jpg"]','pending','2026-06-29 14:00:00');

-- ============================================================
-- 五、预约域 askxuan_booking（booking_status_log/booking_review，booking 表保留在 askxuan 库）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_booking` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_booking`;

CREATE TABLE IF NOT EXISTS `booking_status_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `booking_id` VARCHAR(32) NOT NULL COMMENT '预约单号',
  `from_status` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '原状态',
  `to_status` VARCHAR(32) NOT NULL COMMENT '目标状态',
  `operator_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '操作人ID',
  `operator_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'user/temple_admin/system',
  `remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '备注',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_booking` (`booking_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='预约状态变更日志';

INSERT INTO `booking_status_log` (`booking_id`,`from_status`,`to_status`,`operator_id`,`operator_type`,`remark`,`create_time`) VALUES
('B20260630001','','pending','1001','user','用户创建预约','2026-06-30 08:30:00');

CREATE TABLE IF NOT EXISTS `booking_review` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `booking_id` VARCHAR(32) NOT NULL COMMENT '预约单号',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `rating` INT NOT NULL DEFAULT 5 COMMENT '评分 1-5',
  `content` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '评价内容',
  `images` TEXT COMMENT '评价图片URL，JSON数组',
  `master_reply` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '法师回复',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_booking` (`booking_id`),
  KEY `idx_user` (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='预约评价';

INSERT INTO `booking_review` (`booking_id`,`user_id`,`rating`,`content`,`images`,`master_reply`,`create_time`) VALUES
('B20260615003','1001',5,'白云观化太岁法事非常灵验，清风道长态度慈悲。','["/assets/review-baimasi-1.jpg"]','感恩居士加持，福生无量天尊。','2026-06-21 10:00:00');

-- ============================================================
-- 六、消息域 askxuan_message（message_template/push_log，message 表保留在 askxuan 库）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_message` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_message`;

CREATE TABLE IF NOT EXISTS `message_template` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(64) NOT NULL COMMENT '模板编码',
  `title_template` VARCHAR(128) NOT NULL COMMENT '标题模板',
  `content_template` VARCHAR(512) NOT NULL COMMENT '内容模板',
  `variables` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '变量，JSON数组',
  `type` VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'booking/system/consult/income/audit',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_type` (`type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消息模板';

INSERT INTO `message_template` (`code`,`title_template`,`content_template`,`variables`,`type`,`create_time`) VALUES
('BOOKING_CREATED','预约已创建','您的预约（单号 {bookingId}）已提交，请等待寺院确认。','["bookingId"]','booking','2026-06-28 10:00:00'),
('BOOKING_CONFIRMED','预约已确认','您的预约（单号 {bookingId}）已被寺院确认，请按时到达。','["bookingId"]','booking','2026-06-28 10:00:00'),
('WITHDRAWAL_RESULT','提现审核结果','您的提现申请（单号 {withdrawalId}）{result}。','["withdrawalId","result"]','income','2026-06-28 10:00:00');

CREATE TABLE IF NOT EXISTS `push_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `user_id` VARCHAR(64) NOT NULL COMMENT '接收用户ID',
  `push_type` VARCHAR(32) NOT NULL DEFAULT 'app_push' COMMENT 'app_push/sms',
  `title` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '标题',
  `content` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '内容',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/success/failed',
  `biz_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '业务类型',
  `biz_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '业务ID',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推送日志';

CREATE TABLE IF NOT EXISTS `device_token` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `client_type` VARCHAR(32) NOT NULL DEFAULT 'customer' COMMENT 'customer/master',
  `platform` VARCHAR(16) NOT NULL DEFAULT 'ios' COMMENT 'ios/android',
  `device_token` VARCHAR(255) NOT NULL COMMENT 'APNs/厂商推送 token',
  `bundle_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'iOS bundle id',
  `app_version` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '客户端版本',
  `status` VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active/inactive',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_device_token` (`device_token`),
  KEY `idx_user_client` (`user_id`,`client_type`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设备推送 token';

-- 系统公告表（通信模块 - 系统公告，IM 咨询对话由 OpenIM 独立服务承载）
CREATE TABLE IF NOT EXISTS `system_announcement` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `title` VARCHAR(128) NOT NULL COMMENT '公告标题',
  `content` TEXT NOT NULL COMMENT '公告内容',
  `type` VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT 'system/activity/maintenance',
  `target_audience` VARCHAR(32) NOT NULL DEFAULT 'all' COMMENT 'all/customer/temple_admin/master',
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft' COMMENT 'draft/published/offline',
  `publish_time` DATETIME DEFAULT NULL COMMENT '发布时间',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_status_type` (`status`, `type`),
  KEY `idx_publish_time` (`publish_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统公告';

INSERT INTO `system_announcement` (`title`,`content`,`type`,`target_audience`,`status`,`publish_time`) VALUES
('欢迎使用问玄东方平台', '连接信众与寺院/法师的一站式服务平台，预约祈福、AI问事、DIY手串。','system','all','published','2026-07-01 00:00:00');

-- ============================================================
-- 七、商品域 askxuan_product（product/product_sku/product_category/product_image/intent_tag）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_product` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_product`;

CREATE TABLE IF NOT EXISTS `product_category` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `parent_id` BIGINT NOT NULL DEFAULT 0 COMMENT '父分类ID',
  `name` VARCHAR(64) NOT NULL COMMENT '分类名称',
  `level` INT NOT NULL DEFAULT 1 COMMENT '层级',
  `sort` INT NOT NULL DEFAULT 0 COMMENT '排序',
  PRIMARY KEY (`id`),
  KEY `idx_parent` (`parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品分类';

INSERT INTO `product_category` (`id`,`parent_id`,`name`,`level`,`sort`) VALUES
(1,0,'佛珠手串',1,1),
(2,1,'小叶紫檀',2,1),
(3,1,'星月菩提',2,2),
(4,0,'法器供具',1,2);

CREATE TABLE IF NOT EXISTS `product` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `product_no` VARCHAR(32) NOT NULL COMMENT '商品编码',
  `name` VARCHAR(128) NOT NULL COMMENT '商品名称',
  `category_id` BIGINT NOT NULL DEFAULT 0 COMMENT '分类ID',
  `description` TEXT COMMENT '描述',
  `main_image` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '主图',
  `status` VARCHAR(32) NOT NULL DEFAULT 'draft' COMMENT 'draft/on_shelf/off_shelf',
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '售价',
  `market_price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '划线价',
  `stock` INT NOT NULL DEFAULT 0 COMMENT '库存',
  `tags` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '标签，逗号分隔',
  `freight_template_id` BIGINT NOT NULL DEFAULT 0 COMMENT '运费模板ID',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_product_no` (`product_no`),
  KEY `idx_category` (`category_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品';

INSERT INTO `product` (`product_no`,`name`,`category_id`,`description`,`main_image`,`status`,`price`,`market_price`,`stock`,`tags`,`freight_template_id`) VALUES
('P20260600001','小叶紫檀108颗佛珠',2,'精选印度小叶紫檀，手工打磨，108颗佛珠。','/assets/product-xiaoyezitan.jpg','on_shelf',388.00,588.00,100,'开光,禅修',1),
('P20260600002','星月菩提手串',3,'海南星月菩提，顺白高密，配蜜蜡佛头。','/assets/product-xingyueputi.jpg','on_shelf',198.00,298.00,80,'禅修,供养',1);

CREATE TABLE IF NOT EXISTS `product_sku` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `product_id` BIGINT NOT NULL COMMENT '商品ID',
  `spec_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '规格名',
  `spec_value` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '规格值',
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '价格',
  `stock` INT NOT NULL DEFAULT 0 COMMENT '库存',
  `sku_no` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'SKU编码',
  PRIMARY KEY (`id`),
  KEY `idx_product` (`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品SKU';

INSERT INTO `product_sku` (`product_id`,`spec_name`,`spec_value`,`price`,`stock`,`sku_no`) VALUES
(1,'尺寸','8mm',388.00,50,'SKU-P001-8'),
(1,'尺寸','10mm',428.00,50,'SKU-P001-10'),
(2,'尺寸','10mm',198.00,80,'SKU-P002-10');

CREATE TABLE IF NOT EXISTS `product_image` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `product_id` BIGINT NOT NULL COMMENT '商品ID',
  `image_url` VARCHAR(255) NOT NULL COMMENT '图片URL',
  `sort` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `type` VARCHAR(32) NOT NULL DEFAULT 'detail' COMMENT 'main/detail',
  PRIMARY KEY (`id`),
  KEY `idx_product` (`product_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品图片';

CREATE TABLE IF NOT EXISTS `intent_tag` (
  `code` VARCHAR(32) NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `icon` VARCHAR(64) NOT NULL DEFAULT '',
  `sort` INT NOT NULL DEFAULT 0,
  `status` VARCHAR(16) NOT NULL DEFAULT 'enabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`code`),
  KEY `idx_intent_status_sort` (`status`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='诉求标签';

CREATE TABLE IF NOT EXISTS `product_intent_tag` (
  `product_id` BIGINT NOT NULL,
  `tag_code` VARCHAR(32) NOT NULL,
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`product_id`,`tag_code`),
  KEY `idx_product_intent_code` (`tag_code`),
  CONSTRAINT `fk_intent_product` FOREIGN KEY (`product_id`) REFERENCES `product` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_product_intent_tag` FOREIGN KEY (`tag_code`) REFERENCES `intent_tag` (`code`) ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商品诉求标签映射';

INSERT INTO `intent_tag` (`code`,`name`,`description`,`icon`,`sort`) VALUES
('peace','求平安','祈福、护佑与健康相关商品和服务','shield.lefthalf.filled',10),
('wealth','求财运','财运、供养与事业助力相关商品和服务','banknote.fill',20),
('love','求姻缘','姻缘、人际与家庭相关商品和服务','heart.fill',30),
('career','求事业','事业、风水与开光相关商品和服务','briefcase.fill',40),
('study','求学业','学业、智慧与考试相关商品和服务','book.fill',50),
('taisui','化太岁','本命年与化太岁相关服务','circle.hexagongrid.fill',60),
('diy','定手串','手串材料与定制相关商品','circle.grid.cross.fill',70),
('rite','做法事','超度等法事服务','hands.sparkles.fill',80);

INSERT IGNORE INTO `product_intent_tag` (`product_id`,`tag_code`)
SELECT id, 'diy' FROM `product` WHERE product_no IN ('P20260600001','P20260600002');
INSERT IGNORE INTO `product_intent_tag` (`product_id`,`tag_code`)
SELECT id, 'peace' FROM `product` WHERE product_no='P20260600001';

INSERT INTO `product_image` (`product_id`,`image_url`,`sort`,`type`) VALUES
(1,'/assets/product-xiaoyezitan.jpg',0,'main'),
(1,'/assets/product-xiaoyezitan-2.jpg',1,'detail'),
(2,'/assets/product-xingyueputi.jpg',0,'main');

-- ============================================================
-- 八、DIY域 askxuan_diy（material_sku/diy_design/diy_order/diy_order_item/blessing_task）
-- material 表保留在 askxuan 库
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_diy` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_diy`;

CREATE TABLE IF NOT EXISTS `material` LIKE `askxuan`.`material`;
INSERT INTO `material` (`id`,`name`,`spec`,`unit_price`,`unit`,`category`,`five_elements`,`image`,`stock`,`status`,`create_time`,`update_time`)
SELECT `id`,`name`,`spec`,`unit_price`,`unit`,`category`,`five_elements`,`image`,`stock`,`status`,`create_time`,`update_time` FROM `askxuan`.`material`
ON DUPLICATE KEY UPDATE `name`=VALUES(`name`),`spec`=VALUES(`spec`),`unit_price`=VALUES(`unit_price`),`unit`=VALUES(`unit`),`category`=VALUES(`category`),`five_elements`=VALUES(`five_elements`),`image`=VALUES(`image`),`stock`=VALUES(`stock`),`status`=VALUES(`status`);

CREATE TABLE IF NOT EXISTS `extra_service` LIKE `askxuan`.`extra_service`;
INSERT INTO `extra_service` (`id`,`code`,`name`,`temple_code`,`master_code`,`price`,`description`,`create_time`,`update_time`)
SELECT `id`,`code`,`name`,`temple_code`,`master_code`,`price`,`description`,`create_time`,`update_time` FROM `askxuan`.`extra_service`
ON DUPLICATE KEY UPDATE `name`=VALUES(`name`),`temple_code`=VALUES(`temple_code`),`master_code`=VALUES(`master_code`),`price`=VALUES(`price`),`description`=VALUES(`description`);

CREATE TABLE IF NOT EXISTS `material_sku` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `material_id` BIGINT NOT NULL COMMENT '材料ID',
  `spec` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '规格',
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '价格',
  `stock` INT NOT NULL DEFAULT 0 COMMENT '库存',
  PRIMARY KEY (`id`),
  KEY `idx_material` (`material_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='材料SKU';

INSERT INTO `material_sku` (`material_id`,`spec`,`price`,`stock`) VALUES
(1,'10mm',28.00,500),
(2,'10mm',18.00,500),
(9,'10mm',48.00,200);

CREATE TABLE IF NOT EXISTS `diy_design` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `design_no` VARCHAR(32) NOT NULL COMMENT '设计编码',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设计名称',
  `design_data` TEXT COMMENT '设计数据，JSON',
  `total_price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '总价',
  `status` VARCHAR(32) NOT NULL DEFAULT 'private' COMMENT 'private/public/pending_review/approved/rejected',
  `bless_service_code` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '加持服务编码',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_design_no` (`design_no`),
  KEY `idx_user` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='DIY设计';

INSERT INTO `diy_design` (`design_no`,`user_id`,`name`,`design_data`,`total_price`,`status`,`bless_service_code`,`create_time`) VALUES
('DD20260628001','1001','紫檀开光手串','{"name":"紫檀开光手串","materials":["小叶紫檀圆珠","蜜蜡佛头"]}',336.00,'pending_review','E001','2026-06-28 14:00:00');

CREATE TABLE IF NOT EXISTS `diy_order` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_no` VARCHAR(32) NOT NULL COMMENT '订单号',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `design_id` BIGINT NOT NULL DEFAULT 0 COMMENT '设计ID',
  `material_fee` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '材料费',
  `bless_fee` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '加持费',
  `total_fee` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '总价',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending_review' COMMENT 'pending_review/in_making/awaiting_blessing/blessing_in_progress/blessing_completed/awaiting_shipment/shipped/completed/cancelled/in_return',
  `payment_status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/success/refunded',
  `address_id` BIGINT NOT NULL DEFAULT 0 COMMENT '收货地址ID',
  `source` VARCHAR(16) NOT NULL DEFAULT 'custom' COMMENT 'custom/design_square',
  `creator_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '设计作者ID快照',
  `creator_share_rate` DECIMAL(7,6) NOT NULL DEFAULT 0 COMMENT '作者分成比例快照',
  `original_material_fee` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '设计展示材料费快照',
  `price_changed` TINYINT NOT NULL DEFAULT 0 COMMENT '下单时价格是否变化',
  `design_snapshot` LONGTEXT COMMENT '不可变设计快照JSON',
  `pricing_snapshot` LONGTEXT COMMENT '不可变计价快照JSON',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='DIY订单';

INSERT INTO `diy_order` (`order_no`,`user_id`,`design_id`,`material_fee`,`bless_fee`,`total_fee`,`status`,`address_id`,`create_time`) VALUES
('DIY20260630001','1001',1,336.00,168.00,504.00,'awaiting_blessing',1,'2026-06-30 10:00:00');

CREATE TABLE IF NOT EXISTS `diy_order_item` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_id` BIGINT NOT NULL COMMENT 'DIY订单ID',
  `material_id` BIGINT NOT NULL COMMENT '材料ID',
  `sku_id` BIGINT NOT NULL DEFAULT 0 COMMENT '材料SKU ID，0表示基础规格',
  `material_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '材料名称',
  `spec` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '规格',
  `unit_price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '单价',
  `quantity` INT NOT NULL DEFAULT 0 COMMENT '数量',
  `subtype` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '子类型',
  PRIMARY KEY (`id`),
  KEY `idx_order` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='DIY订单明细';

INSERT INTO `diy_order_item` (`order_id`,`material_id`,`material_name`,`spec`,`unit_price`,`quantity`,`subtype`) VALUES
(1,1,'小叶紫檀圆珠','10mm',28.00,10,'main_bead'),
(1,10,'蜜蜡佛头','12mm',68.00,1,'buddha_head');

CREATE TABLE IF NOT EXISTS `diy_config` (
  `config_key` VARCHAR(64) NOT NULL,
  `config_value` VARCHAR(255) NOT NULL DEFAULT '',
  `description` VARCHAR(255) NOT NULL DEFAULT '',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='DIY业务配置';

INSERT INTO `diy_config` (`config_key`,`config_value`,`description`) VALUES
('diy_design_creator_share','0','设计广场作者分成比例，范围0-1，默认0')
ON DUPLICATE KEY UPDATE `description`=VALUES(`description`);

CREATE TABLE IF NOT EXISTS `diy_creator_earning` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `earning_no` VARCHAR(32) NOT NULL,
  `order_id` BIGINT NOT NULL,
  `order_no` VARCHAR(32) NOT NULL,
  `design_id` BIGINT NOT NULL,
  `creator_id` VARCHAR(64) NOT NULL,
  `payment_no` VARCHAR(32) NOT NULL DEFAULT '',
  `base_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  `share_rate` DECIMAL(7,6) NOT NULL DEFAULT 0,
  `earning_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/settled/cancelled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_earning_no` (`earning_no`),
  UNIQUE KEY `uk_order` (`order_id`),
  KEY `idx_creator_status` (`creator_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='设计广场作者收益';

CREATE TABLE IF NOT EXISTS `blessing_task` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `task_no` VARCHAR(32) NOT NULL COMMENT '任务编码',
  `diy_order_no` VARCHAR(32) NOT NULL COMMENT 'DIY订单号',
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `master_code` VARCHAR(16) NOT NULL DEFAULT '' COMMENT '法师编码',
  `status` VARCHAR(32) NOT NULL DEFAULT 'dispatched' COMMENT 'dispatched/assigned/accepted/in_progress/completed/rejected',
  `certificate_urls` TEXT COMMENT '加持证书URL，JSON数组',
  `assign_time` DATETIME DEFAULT NULL COMMENT '分配时间',
  `complete_time` DATETIME DEFAULT NULL COMMENT '完成时间',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_task_no` (`task_no`),
  KEY `idx_temple` (`temple_code`),
  KEY `idx_master` (`master_code`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='加持任务';

INSERT INTO `blessing_task` (`task_no`,`diy_order_no`,`temple_code`,`master_code`,`status`,`create_time`) VALUES
('BT20260630001','DIY20260630001','T001','','dispatched','2026-06-30 10:00:00');

-- ============================================================
-- 九、订单域 askxuan_shop（shop_order/shop_order_item/shop_order_logistics/return_order）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_shop` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_shop`;

CREATE TABLE IF NOT EXISTS `shop_order` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_no` VARCHAR(32) NOT NULL COMMENT '订单号',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `total_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '订单总额',
  `pay_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '实付金额',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending_payment' COMMENT 'pending_payment/paid/shipped/completed/cancelled/in_return',
  `address_id` BIGINT NOT NULL DEFAULT 0 COMMENT '收货地址ID',
  `note` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '备注',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商城订单';

INSERT INTO `shop_order` (`order_no`,`user_id`,`total_amount`,`pay_amount`,`status`,`address_id`,`create_time`) VALUES
('SO20260620001','1001',388.00,388.00,'shipped',1,'2026-06-20 09:00:00');

CREATE TABLE IF NOT EXISTS `shop_order_item` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_id` BIGINT NOT NULL COMMENT '订单ID',
  `product_id` BIGINT NOT NULL COMMENT '商品ID',
  `sku_id` BIGINT NOT NULL DEFAULT 0 COMMENT 'SKU ID',
  `product_name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '商品名称',
  `sku_spec` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '规格',
  `price` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '单价',
  `quantity` INT NOT NULL DEFAULT 0 COMMENT '数量',
  `image` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '商品图片',
  PRIMARY KEY (`id`),
  KEY `idx_order` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='商城订单明细';

INSERT INTO `shop_order_item` (`order_id`,`product_id`,`sku_id`,`product_name`,`sku_spec`,`price`,`quantity`,`image`) VALUES
(1,1,1,'小叶紫檀108颗佛珠','8mm',388.00,1,'/assets/product-xiaoyezitan.jpg');

CREATE TABLE IF NOT EXISTS `shop_order_logistics` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `order_id` BIGINT NOT NULL COMMENT '订单ID',
  `express_company` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '快递公司',
  `tracking_no` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '物流单号',
  `ship_time` DATETIME DEFAULT NULL COMMENT '发货时间',
  PRIMARY KEY (`id`),
  KEY `idx_order` (`order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='订单物流';

INSERT INTO `shop_order_logistics` (`order_id`,`express_company`,`tracking_no`,`ship_time`) VALUES
(1,'顺丰速运','SF1234567890','2026-06-20 10:00:00');

CREATE TABLE IF NOT EXISTS `return_order` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `return_no` VARCHAR(32) NOT NULL COMMENT '退换货单号',
  `order_id` BIGINT NOT NULL COMMENT '订单ID',
  `type` VARCHAR(32) NOT NULL DEFAULT 'return' COMMENT 'return/exchange',
  `reason` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '原因',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending_review' COMMENT 'pending_review/approved/return_shipping/return_received/refunding/completed/rejected',
  `refund_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '退款金额',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_return_no` (`return_no`),
  KEY `idx_order` (`order_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='退换货';

-- ============================================================
-- 十、支付域 askxuan_shop（payment/payment_log/refund）
-- ============================================================
USE `askxuan_shop`;

CREATE TABLE IF NOT EXISTS `payment` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `payment_no` VARCHAR(32) NOT NULL COMMENT '支付单号',
  `user_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '用户ID',
  `order_type` VARCHAR(32) NOT NULL COMMENT 'booking/shop_order/diy_order',
  `order_no` VARCHAR(32) NOT NULL COMMENT '业务订单号',
  `amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '金额',
  `channel` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'wechat/alipay',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/success/failed/refunding/refunded/closed',
  `trade_no` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '第三方交易号',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_payment_no` (`payment_no`),
  KEY `idx_order` (`order_type`,`order_no`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付单';

INSERT INTO `payment` (`payment_no`,`user_id`,`order_type`,`order_no`,`amount`,`channel`,`status`,`trade_no`,`create_time`) VALUES
('PAY20260620001','1001','shop_order','SO20260620001',388.00,'wechat','success','420000000020260620001','2026-06-20 09:05:00');

CREATE TABLE IF NOT EXISTS `payment_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `payment_id` BIGINT NOT NULL COMMENT '支付单ID',
  `action` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '动作 create/notify/refund',
  `request` TEXT COMMENT '请求报文',
  `response` TEXT COMMENT '响应报文',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_payment` (`payment_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='支付日志';

CREATE TABLE IF NOT EXISTS `refund` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `refund_no` VARCHAR(32) NOT NULL COMMENT '退款单号',
  `payment_id` BIGINT NOT NULL COMMENT '支付单ID',
  `amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '退款金额',
  `reason` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '退款原因',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/processing/success/failed',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_refund_no` (`refund_no`),
  KEY `idx_payment` (`payment_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='退款';

-- ============================================================
-- 十一、财务域 askxuan_finance（settlement/withdrawal/commission_config/finance_log）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_finance` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_finance`;

CREATE TABLE IF NOT EXISTS `commission_config` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `biz_type` VARCHAR(32) NOT NULL COMMENT '业务类型 booking/diy_blessing/diy_material/shop_order',
  `rate` DECIMAL(5,4) NOT NULL DEFAULT 0.0000 COMMENT '抽成比例',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_biz_type` (`biz_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='抽成配置';

INSERT INTO `commission_config` (`biz_type`,`rate`,`description`,`update_time`) VALUES
('booking',0.1500,'预约服务平台抽成15%','2026-07-01 00:00:00'),
('diy_blessing',0.1500,'DIY加持费平台抽成15%','2026-07-01 00:00:00'),
('diy_material',0.1000,'DIY材料费平台抽成10%','2026-07-01 00:00:00'),
('shop_order',0.1000,'商城订单平台抽成10%','2026-07-01 00:00:00');

CREATE TABLE IF NOT EXISTS `settlement` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `settlement_no` VARCHAR(32) NOT NULL COMMENT '结算单号',
  `settle_type` VARCHAR(32) NOT NULL COMMENT 'temple/master/shop',
  `target_id` VARCHAR(32) NOT NULL COMMENT '结算对象编码',
  `target_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '结算对象名称',
  `period_start` DATETIME DEFAULT NULL COMMENT '周期开始',
  `period_end` DATETIME DEFAULT NULL COMMENT '周期结束',
  `order_count` INT NOT NULL DEFAULT 0 COMMENT '订单数',
  `total_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '订单总额',
  `commission_rate` DECIMAL(5,4) NOT NULL DEFAULT 0.0000 COMMENT '抽成比例',
  `commission_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '抽成金额',
  `settle_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '应结金额',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/confirmed/paid',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_settlement_no` (`settlement_no`),
  KEY `idx_type_status` (`settle_type`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='结算单';

INSERT INTO `settlement` (`settlement_no`,`settle_type`,`target_id`,`target_name`,`period_start`,`period_end`,`order_count`,`total_amount`,`commission_rate`,`commission_amount`,`settle_amount`,`status`,`create_time`) VALUES
('SET2026060001','temple','T001','灵隐寺','2026-06-01 00:00:00','2026-06-30 23:59:59',15,3500.00,0.1500,525.00,2975.00,'confirmed','2026-07-01 02:00:00'),
('SET2026060002','master','M001','智海法师','2026-06-01 00:00:00','2026-06-30 23:59:59',12,2800.00,0.1500,420.00,2380.00,'pending','2026-07-01 02:00:00'),
('SET2026060003','shop','SHOP001','东方商城','2026-06-01 00:00:00','2026-06-30 23:59:59',86,25600.00,0.1000,2560.00,23040.00,'paid','2026-07-01 02:00:00');

CREATE TABLE IF NOT EXISTS `withdrawal` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `withdrawal_no` VARCHAR(32) NOT NULL COMMENT '提现单号',
  `applicant_type` VARCHAR(32) NOT NULL COMMENT 'temple/master/shop',
  `applicant_id` VARCHAR(32) NOT NULL COMMENT '申请人编码',
  `amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '提现金额',
  `bank_card` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '银行卡号',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/processing/success/failed/rejected',
  `audit_time` DATETIME DEFAULT NULL COMMENT '审核时间',
  `process_time` DATETIME DEFAULT NULL COMMENT '打款时间',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_withdrawal_no` (`withdrawal_no`),
  KEY `idx_type_status` (`applicant_type`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='提现申请';

INSERT INTO `withdrawal` (`withdrawal_no`,`applicant_type`,`applicant_id`,`amount`,`bank_card`,`status`,`audit_time`,`process_time`,`create_time`) VALUES
('WD20260701001','temple','T001',2000.00,'6222021234567890','pending',NULL,NULL,'2026-07-01 09:00:00'),
('WD20260628002','master','M001',1500.00,'6222020987654321','success','2026-06-28 14:00:00','2026-06-29 10:00:00','2026-06-28 10:00:00');

CREATE TABLE IF NOT EXISTS `finance_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `settlement_id` BIGINT NOT NULL DEFAULT 0 COMMENT '结算单ID',
  `amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '金额',
  `type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '类型 income/expense',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_settlement` (`settlement_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='财务日志';

-- ============================================================
-- 十二、评价域 askxuan_review（review/review_reply/review_report）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_review` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_review`;

CREATE TABLE IF NOT EXISTS `review` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `review_no` VARCHAR(32) NOT NULL COMMENT '评价单号',
  `user_id` VARCHAR(64) NOT NULL COMMENT '评价人ID',
  `target_type` VARCHAR(32) NOT NULL COMMENT 'booking/diy_order/shop_order',
  `target_id` VARCHAR(64) NOT NULL COMMENT '目标ID',
  `rating` INT NOT NULL DEFAULT 5 COMMENT '评分 1-5',
  `content` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '评价内容',
  `images` TEXT COMMENT '评价图片URL，JSON数组',
  `status` VARCHAR(32) NOT NULL DEFAULT 'normal' COMMENT 'normal/hidden',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_review_no` (`review_no`),
  KEY `idx_target` (`target_type`,`target_id`),
  KEY `idx_user` (`user_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评价';

INSERT INTO `review` (`review_no`,`user_id`,`target_type`,`target_id`,`rating`,`content`,`images`,`status`,`create_time`) VALUES
('RV20260620001','1001','booking','B20260615003',5,'清风道长非常专业，化太岁仪式庄重，感觉很安心。','["https://oss.askxuan.com/rv/1.jpg"]','normal','2026-06-20 18:00:00'),
('RV20260625002','1002','booking','B20260628002',4,'释延心法师超度法事很用心，整体体验不错。','[]','normal','2026-06-25 20:30:00'),
('RV20260628003','1001','shop_order','SO20260620001',5,'小叶紫檀手串品质很好，包装精美，非常满意！','["https://oss.askxuan.com/rv/2.jpg","https://oss.askxuan.com/rv/3.jpg"]','normal','2026-06-28 10:00:00');

CREATE TABLE IF NOT EXISTS `review_reply` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `review_id` BIGINT NOT NULL COMMENT '评价ID',
  `replier_type` VARCHAR(32) NOT NULL COMMENT 'temple_admin/master/platform',
  `replier_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '回复人ID',
  `content` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '回复内容',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_review` (`review_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评价回复';

INSERT INTO `review_reply` (`review_id`,`replier_type`,`replier_id`,`content`,`create_time`) VALUES
(1,'master','M002','感谢您的评价，愿福生无量。','2026-06-20 19:00:00');

CREATE TABLE IF NOT EXISTS `review_report` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `review_id` BIGINT NOT NULL COMMENT '评价ID',
  `reporter_id` VARCHAR(64) NOT NULL COMMENT '举报人ID',
  `reason` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '举报原因',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/handled/rejected',
  `handle_result` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '处理结果',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_review` (`review_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='评价举报';

INSERT INTO `review_report` (`review_id`,`reporter_id`,`reason`,`status`,`create_time`) VALUES
(2,'T003','评价内容涉及不实信息','pending','2026-06-26 09:00:00');

-- ============================================================
-- 十三、审核域 askxuan_audit（audit_queue/audit_log/report/sensitive_word）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_audit` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_audit`;

CREATE TABLE IF NOT EXISTS `audit_queue` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `biz_type` VARCHAR(32) NOT NULL COMMENT 'design/temple/master/comment',
  `biz_id` VARCHAR(64) NOT NULL COMMENT '业务ID',
  `submitter_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '提交人ID',
  `content_snapshot` TEXT COMMENT '内容快照，JSON',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected/first_pass/final_pass/pass',
  `auditor_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '审核人ID',
  `audit_time` DATETIME DEFAULT NULL COMMENT '审核时间',
  `audit_remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '审核备注',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_biz` (`biz_type`,`biz_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核队列';

INSERT INTO `audit_queue` (`biz_type`,`biz_id`,`submitter_id`,`content_snapshot`,`status`,`create_time`) VALUES
('design','DD20260628001','1001','{"name":"紫檀开光手串","materials":["小叶紫檀","蜜蜡佛头"]}','pending','2026-06-28 14:00:00'),
('temple','T005','T005','{"name":"普陀山","type":"汉传佛教","status":"待审核"}','pending','2026-06-29 10:00:00'),
('master','M005','T005','{"name":"慧明法师","credential":"戒牒编号XXX"}','pending','2026-06-29 11:00:00'),
('design','DD20260620002','1002','{"name":"星月菩提手串","materials":["星月菩提","白水晶隔片"]}','approved','2026-06-20 15:00:00');

CREATE TABLE IF NOT EXISTS `audit_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `audit_id` BIGINT NOT NULL COMMENT '审核队列ID',
  `action` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '动作 approve/reject/submit',
  `operator_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '操作人ID',
  `remark` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '备注',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_audit` (`audit_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='审核日志';

INSERT INTO `audit_log` (`audit_id`,`action`,`operator_id`,`remark`,`create_time`) VALUES
(4,'approve','ADMIN001','设计审核通过','2026-06-20 16:00:00');

CREATE TABLE IF NOT EXISTS `report` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `reporter_id` VARCHAR(64) NOT NULL COMMENT '举报人ID',
  `target_type` VARCHAR(32) NOT NULL COMMENT 'design/comment/master/temple',
  `target_id` VARCHAR(64) NOT NULL COMMENT '目标ID',
  `reason` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '举报原因',
  `evidence_urls` TEXT COMMENT '证据图片URL，JSON数组',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/handled/rejected',
  `handler_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '处理人ID',
  `handle_result` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '处理结果',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_target` (`target_type`,`target_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='举报';

INSERT INTO `report` (`reporter_id`,`target_type`,`target_id`,`reason`,`evidence_urls`,`status`,`handler_id`,`handle_result`,`create_time`) VALUES
('1002','design','DD20260615001','设计涉及侵权','["https://oss.askxuan.com/report/1.jpg"]','pending','','','2026-06-26 09:00:00'),
('1001','comment','RV20260610005','评论内容含不当言论','[]','handled','ADMIN001','已删除违规评论','2026-06-22 14:00:00');

CREATE TABLE IF NOT EXISTS `sensitive_word` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `word` VARCHAR(64) NOT NULL COMMENT '敏感词',
  `category` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '分类 religious/political/vulgar/advertising',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_word` (`word`),
  KEY `idx_category_status` (`category`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='敏感词';

INSERT INTO `sensitive_word` (`word`,`category`,`status`,`create_time`) VALUES
('邪教','religious','enabled','2026-07-01 00:00:00'),
('反动','political','enabled','2026-07-01 00:00:00'),
('色情','vulgar','enabled','2026-07-01 00:00:00'),
('加微信','advertising','enabled','2026-07-01 00:00:00'),
('代购','advertising','disabled','2026-07-01 00:00:00');

-- ============================================================
-- 十四、物流域 askxuan_logistics（express_company/freight_template/logistics_track）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_logistics` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_logistics`;

CREATE TABLE IF NOT EXISTS `express_company` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(32) NOT NULL COMMENT '快递公司编码',
  `name` VARCHAR(64) NOT NULL COMMENT '名称',
  `logo_url` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'LOGO',
  `customer_service` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '客服电话',
  `sort` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='快递公司';

INSERT INTO `express_company` (`code`,`name`,`logo_url`,`customer_service`,`sort`,`status`,`create_time`) VALUES
('SF','顺丰速运','https://oss.askxuan.com/express/sf.png','95338',1,'enabled','2026-07-01 00:00:00'),
('ZTO','中通快递','https://oss.askxuan.com/express/zto.png','95311',2,'enabled','2026-07-01 00:00:00'),
('YTO','圆通速递','https://oss.askxuan.com/express/yto.png','95554',3,'enabled','2026-07-01 00:00:00'),
('EMS','EMS','https://oss.askxuan.com/express/ems.png','11183',4,'enabled','2026-07-01 00:00:00');

CREATE TABLE IF NOT EXISTS `freight_template` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL COMMENT '模板名称',
  `type` VARCHAR(32) NOT NULL DEFAULT 'by_weight' COMMENT 'by_weight/by_piece',
  `free_shipping` TINYINT NOT NULL DEFAULT 0 COMMENT '0否 1包邮',
  `config` TEXT COMMENT '计费配置，JSON',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运费模板';

INSERT INTO `freight_template` (`name`,`type`,`free_shipping`,`config`,`status`,`create_time`) VALUES
('全国包邮','by_weight',1,'{"default":{"firstWeight":1,"firstFee":0,"continueWeight":1,"continueFee":0}}','enabled','2026-07-01 00:00:00'),
('偏远地区加价','by_weight',0,'{"default":{"firstWeight":1,"firstFee":8,"continueWeight":1,"continueFee":2},"remote":{"firstWeight":1,"firstFee":15,"continueWeight":1,"continueFee":5}}','enabled','2026-07-01 00:00:00');

CREATE TABLE IF NOT EXISTS `logistics_track` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `tracking_no` VARCHAR(64) NOT NULL COMMENT '物流单号',
  `express_code` VARCHAR(32) NOT NULL COMMENT '快递公司编码',
  `express_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '快递公司名称',
  `biz_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'order/diy',
  `biz_no` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '业务单号',
  `status` VARCHAR(32) NOT NULL DEFAULT 'pending' COMMENT 'pending/in_transit/delivered/signed',
  `traces` TEXT COMMENT '轨迹节点，JSON数组',
  `last_sync_time` DATETIME DEFAULT NULL COMMENT '最后同步时间',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tracking_no` (`tracking_no`),
  KEY `idx_biz` (`biz_type`,`biz_no`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='物流轨迹';

INSERT INTO `logistics_track` (`tracking_no`,`express_code`,`express_name`,`biz_type`,`biz_no`,`status`,`traces`,`last_sync_time`,`create_time`,`update_time`) VALUES
('SF1234567890','SF','顺丰速运','order','SO20260620001','in_transit','[{"time":"2026-06-20 10:00:00","desc":"顺丰已揽收"},{"time":"2026-06-20 18:00:00","desc":"快件已到达杭州转运中心"}]','2026-06-20 18:00:00','2026-06-20 10:00:00','2026-06-20 18:00:00'),
('ZTO9876543210','ZTO','中通快递','diy','DIY20260615005','signed','[{"time":"2026-06-15 09:00:00","desc":"中通已揽收"},{"time":"2026-06-16 12:00:00","desc":"快件已派送"},{"time":"2026-06-16 15:30:00","desc":"已签收，本人签收"}]','2026-06-16 15:30:00','2026-06-15 09:00:00','2026-06-16 15:30:00');

-- ============================================================
-- 十五、营销域 askxuan_marketing（coupon/coupon_record/activity/banner/recommend）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_marketing` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_marketing`;

CREATE TABLE IF NOT EXISTS `banner` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `title` VARCHAR(64) NOT NULL COMMENT '标题',
  `image_url` VARCHAR(255) NOT NULL COMMENT '图片URL',
  `link_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'temple/master/product/diy/ad_landing',
  `link_value` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '跳转目标',
  `sort` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `start_time` DATETIME DEFAULT NULL COMMENT '开始时间',
  `end_time` DATETIME DEFAULT NULL COMMENT '结束时间',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_status_sort` (`status`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Banner';

INSERT INTO `banner` (`title`,`image_url`,`link_type`,`link_value`,`sort`,`status`,`start_time`,`end_time`,`create_time`) VALUES
('灵隐寺祈福法会','/banners/lingyin.jpg','temple','T001',1,'enabled','2026-07-01 00:00:00','2026-07-31 23:59:59','2026-06-28 10:00:00'),
('新人首单立减','/banners/newuser.jpg','ad_landing','/promo/newuser',2,'enabled','2026-07-01 00:00:00','2026-12-31 23:59:59','2026-06-28 10:05:00');

CREATE TABLE IF NOT EXISTS `recommend` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `type` VARCHAR(32) NOT NULL COMMENT 'temple/master/product',
  `target_id` VARCHAR(32) NOT NULL COMMENT '目标ID',
  `sort` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_type_status` (`type`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='推荐位';

INSERT INTO `recommend` (`type`,`target_id`,`sort`,`status`,`create_time`) VALUES
('temple','T001',1,'enabled','2026-06-28 10:00:00'),
('temple','T003',2,'enabled','2026-06-28 10:00:00'),
('master','M001',1,'enabled','2026-06-28 10:00:00');

CREATE TABLE IF NOT EXISTS `activity` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `name` VARCHAR(64) NOT NULL COMMENT '活动名称',
  `type` VARCHAR(32) NOT NULL COMMENT 'limited_discount/festival/temple_event',
  `start_time` DATETIME DEFAULT NULL COMMENT '开始时间',
  `end_time` DATETIME DEFAULT NULL COMMENT '结束时间',
  `config` TEXT COMMENT '活动配置，JSON',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='营销活动';

INSERT INTO `activity` (`name`,`type`,`start_time`,`end_time`,`config`,`status`,`create_time`) VALUES
('中元节祈福活动','festival','2026-08-01 00:00:00','2026-08-15 23:59:59','{"discount":0.9}','enabled','2026-06-28 10:00:00'),
('灵隐寺盂兰盆法会','temple_event','2026-08-10 09:00:00','2026-08-10 17:00:00','{"templeId":"T001"}','enabled','2026-06-28 10:00:00');

CREATE TABLE IF NOT EXISTS `coupon` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `coupon_no` VARCHAR(32) NOT NULL COMMENT '优惠券编码',
  `name` VARCHAR(64) NOT NULL COMMENT '名称',
  `type` VARCHAR(32) NOT NULL COMMENT 'full_reduce/discount/new_user/category',
  `value` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '面额/折扣',
  `min_amount` DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '使用门槛',
  `category_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '限定分类',
  `start_time` DATETIME DEFAULT NULL COMMENT '开始时间',
  `end_time` DATETIME DEFAULT NULL COMMENT '结束时间',
  `total_count` INT NOT NULL DEFAULT 0 COMMENT '发放总量',
  `received_count` INT NOT NULL DEFAULT 0 COMMENT '已领取数',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_coupon_no` (`coupon_no`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='优惠券';

INSERT INTO `coupon` (`coupon_no`,`name`,`type`,`value`,`min_amount`,`start_time`,`end_time`,`total_count`,`received_count`,`status`,`create_time`) VALUES
('C20260700001','新人满100减20','new_user',20.00,100.00,'2026-07-01 00:00:00','2026-12-31 23:59:59',1000,12,'enabled','2026-06-28 10:00:00'),
('C20260700002','法事8折券','discount',0.80,0.00,'2026-07-01 00:00:00','2026-08-31 23:59:59',500,3,'enabled','2026-06-28 10:00:00');

CREATE TABLE IF NOT EXISTS `coupon_record` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `coupon_id` BIGINT NOT NULL COMMENT '优惠券ID',
  `coupon_no` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '优惠券编码',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `status` VARCHAR(32) NOT NULL DEFAULT 'unused' COMMENT 'unused/used/expired',
  `order_no` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '核销订单号',
  `use_time` DATETIME DEFAULT NULL COMMENT '核销时间',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_user_status` (`user_id`,`status`),
  KEY `idx_coupon` (`coupon_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='领券记录';

INSERT INTO `coupon_record` (`coupon_id`,`coupon_no`,`user_id`,`status`,`create_time`) VALUES
(1,'C20260700001','1001','unused','2026-06-29 09:00:00');

-- ============================================================
-- 十六、AI域 askxuan_ai（ai_session/ai_message/ai_skill）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_ai` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_ai`;

CREATE TABLE IF NOT EXISTS `ai_skill` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `code` VARCHAR(32) NOT NULL COMMENT '技能编码',
  `name` VARCHAR(64) NOT NULL COMMENT '技能名称',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
  `icon` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '图标',
  `prompt_template` TEXT COMMENT '提示词模板',
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

CREATE TABLE IF NOT EXISTS `ai_session` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `session_no` VARCHAR(32) NOT NULL COMMENT '会话编码',
  `user_id` VARCHAR(64) NOT NULL COMMENT '用户ID',
  `skill_code` VARCHAR(32) NOT NULL COMMENT '技能编码',
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
('AI20260630001','1001','bazi','今年的事业运势','active','2026-06-30 09:00:00','2026-06-30 09:00:00');

CREATE TABLE IF NOT EXISTS `ai_message` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `session_id` BIGINT NOT NULL COMMENT '会话ID',
  `role` VARCHAR(32) NOT NULL COMMENT 'user/assistant',
  `content` TEXT COMMENT '消息内容',
  `tokens` INT NOT NULL DEFAULT 0 COMMENT 'token消耗',
  `status` VARCHAR(16) NOT NULL DEFAULT 'completed' COMMENT 'pending/completed/failed',
  `error_message` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Provider失败原因',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '重试次数',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_session` (`session_id`),
  KEY `idx_message_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI消息';

INSERT INTO `ai_message` (`session_id`,`role`,`content`,`tokens`,`status`,`create_time`) VALUES
(1,'user','请帮我看看今年的事业运势',0,'completed','2026-06-30 09:01:00'),
(1,'assistant','好的，请问您的出生年月日时是？',0,'completed','2026-06-30 09:01:05');

-- ============================================================
-- 十七、媒体域 askxuan_media（media_asset/live_room）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_media` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_media`;

CREATE TABLE IF NOT EXISTS `media_asset` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `media_no` VARCHAR(32) NOT NULL,
  `owner_id` VARCHAR(64) NOT NULL,
  `media_type` VARCHAR(16) NOT NULL COMMENT 'image/video/audio',
  `content_type` VARCHAR(128) NOT NULL DEFAULT '',
  `file_name` VARCHAR(255) NOT NULL DEFAULT '',
  `object_name` VARCHAR(512) NOT NULL,
  `provider` VARCHAR(32) NOT NULL DEFAULT 'local_minio',
  `provider_task_id` VARCHAR(128) NOT NULL DEFAULT '',
  `status` VARCHAR(16) NOT NULL DEFAULT 'uploading' COMMENT 'uploading/uploaded/processing/ready/failed',
  `audit_status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/approved/rejected',
  `playback_url` VARCHAR(1000) NOT NULL DEFAULT '',
  `cover_url` VARCHAR(1000) NOT NULL DEFAULT '',
  `cover_media_id` BIGINT NOT NULL DEFAULT 0,
  `duration` DECIMAL(10,3) NOT NULL DEFAULT 0,
  `file_size` BIGINT NOT NULL DEFAULT 0,
  `error_message` VARCHAR(500) NOT NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_media_no` (`media_no`),
  UNIQUE KEY `uk_object_name` (`object_name`),
  KEY `idx_owner_status` (`owner_id`,`status`),
  KEY `idx_audit_status` (`audit_status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='统一媒体资产';

CREATE TABLE IF NOT EXISTS `live_room` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `room_no` VARCHAR(32) NOT NULL,
  `owner_id` VARCHAR(64) NOT NULL,
  `master_id` VARCHAR(64) NOT NULL,
  `title` VARCHAR(120) NOT NULL,
  `cover_media_id` BIGINT NOT NULL DEFAULT 0,
  `provider` VARCHAR(32) NOT NULL DEFAULT 'disabled',
  `status` VARCHAR(16) NOT NULL DEFAULT 'created' COMMENT 'created/live/ended/failed',
  `openim_group_id` VARCHAR(128) NOT NULL DEFAULT '',
  `push_url` VARCHAR(1000) NOT NULL DEFAULT '',
  `watch_url` VARCHAR(1000) NOT NULL DEFAULT '',
  `provider_room_id` VARCHAR(128) NOT NULL DEFAULT '',
  `started_at` DATETIME NULL,
  `ended_at` DATETIME NULL,
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_room_no` (`room_no`),
  KEY `idx_master_status` (`master_id`,`status`),
  KEY `idx_openim_group` (`openim_group_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播房间';

-- ============================================================
-- 十八、社区域 askxuan_community（post/post_asset/post_like/post_comment/master_follow）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_community` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_community`;

CREATE TABLE IF NOT EXISTS `post` (
  `id` BIGINT NOT NULL AUTO_INCREMENT, `post_no` VARCHAR(32) NOT NULL, `master_id` VARCHAR(64) NOT NULL, `owner_id` VARCHAR(64) NOT NULL,
  `type` VARCHAR(16) NOT NULL, `title` VARCHAR(120) NOT NULL, `content` TEXT, `cover_media_id` BIGINT NOT NULL DEFAULT 0,
  `belief_code` VARCHAR(32) NOT NULL DEFAULT '', `status` VARCHAR(16) NOT NULL DEFAULT 'draft', `audit_id` BIGINT NOT NULL DEFAULT 0,
  `audit_remark` VARCHAR(255) NOT NULL DEFAULT '', `like_count` BIGINT NOT NULL DEFAULT 0, `comment_count` BIGINT NOT NULL DEFAULT 0,
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_post_no` (`post_no`), KEY `idx_feed` (`status`,`create_time`), KEY `idx_master_status` (`master_id`,`status`), KEY `idx_belief` (`belief_code`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='大师广场帖子';
CREATE TABLE IF NOT EXISTS `post_asset` (
  `id` BIGINT NOT NULL AUTO_INCREMENT, `post_no` VARCHAR(32) NOT NULL, `media_id` BIGINT NOT NULL, `asset_type` VARCHAR(16) NOT NULL, `sort` INT NOT NULL DEFAULT 0,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_post_media` (`post_no`,`media_id`), KEY `idx_post_sort` (`post_no`,`sort`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子媒体引用';
CREATE TABLE IF NOT EXISTS `post_like` (`post_no` VARCHAR(32) NOT NULL, `user_id` VARCHAR(64) NOT NULL, `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (`post_no`,`user_id`), KEY `idx_user` (`user_id`,`create_time`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子点赞';
CREATE TABLE IF NOT EXISTS `post_comment` (
  `id` BIGINT NOT NULL AUTO_INCREMENT, `comment_no` VARCHAR(32) NOT NULL, `post_no` VARCHAR(32) NOT NULL, `user_id` VARCHAR(64) NOT NULL, `content` VARCHAR(500) NOT NULL,
  `status` VARCHAR(16) NOT NULL DEFAULT 'pending', `audit_id` BIGINT NOT NULL DEFAULT 0, `audit_remark` VARCHAR(255) NOT NULL DEFAULT '',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`), UNIQUE KEY `uk_comment_no` (`comment_no`), KEY `idx_post_status` (`post_no`,`status`,`create_time`), KEY `idx_audit` (`status`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='帖子评论';
CREATE TABLE IF NOT EXISTS `master_follow` (`master_id` VARCHAR(64) NOT NULL, `user_id` VARCHAR(64) NOT NULL, `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (`master_id`,`user_id`), KEY `idx_user` (`user_id`,`create_time`)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户关注大师';

-- ============================================================
-- 十九、系统域 askxuan_system（operation_log/dictionary/system_config）
-- ============================================================
CREATE DATABASE IF NOT EXISTS `askxuan_system` DEFAULT CHARACTER SET utf8mb4;
USE `askxuan_system`;

CREATE TABLE IF NOT EXISTS `operation_log` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `operator_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '操作人ID',
  `operator_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'admin/user/system',
  `module` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '模块',
  `action` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '动作',
  `target_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '目标类型',
  `target_id` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '目标ID',
  `request` TEXT COMMENT '请求参数',
  `response` TEXT COMMENT '响应结果',
  `ip` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'IP地址',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_operator` (`operator_id`),
  KEY `idx_module` (`module`),
  KEY `idx_create_time` (`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='操作日志';

CREATE TABLE IF NOT EXISTS `dictionary` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `dict_code` VARCHAR(64) NOT NULL COMMENT '字典编码',
  `dict_name` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '字典名称',
  `item_key` VARCHAR(64) NOT NULL COMMENT '项键',
  `item_value` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '项值',
  `sort` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `status` VARCHAR(32) NOT NULL DEFAULT 'enabled' COMMENT 'enabled/disabled',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_dict_code` (`dict_code`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='数据字典';

INSERT INTO `dictionary` (`dict_code`,`dict_name`,`item_key`,`item_value`,`sort`,`status`) VALUES
('booking_status','预约状态','pending','待确认',1,'enabled'),
('booking_status','预约状态','confirmed','已确认',2,'enabled'),
('booking_status','预约状态','in_progress','进行中',3,'enabled'),
('booking_status','预约状态','completed','已完成',4,'enabled'),
('booking_status','预约状态','cancelled','已取消',5,'enabled'),
('booking_status','预约状态','reviewed','已评价',6,'enabled'),
('temple_type','寺院类型','汉传佛教','汉传佛教',1,'enabled'),
('temple_type','寺院类型','道教','道教',2,'enabled'),
('temple_type','寺院类型','藏传佛教','藏传佛教',3,'enabled'),
('payment_channel','支付渠道','wechat','微信支付',1,'enabled'),
('payment_channel','支付渠道','alipay','支付宝',2,'enabled');

CREATE TABLE IF NOT EXISTS `system_config` (
  `id` BIGINT NOT NULL AUTO_INCREMENT,
  `config_key` VARCHAR(128) NOT NULL COMMENT '配置键',
  `config_value` TEXT COMMENT '配置值',
  `description` VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
  `update_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_key` (`config_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统配置';

INSERT INTO `system_config` (`config_key`,`config_value`,`description`) VALUES
('site_name','问玄东方','站点名称'),
('booking_auto_cancel_minutes','30','预约未确认自动取消分钟数'),
('withdrawal_min_amount','100','最低提现金额'),
('review_auto_publish','1','评价是否自动发布 0否 1是'),
('diy_audit_enabled','1','DIY设计是否需要审核 0否 1是');

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================================
-- 初始化完成
-- 默认库 askxuan：9 张 MVP-1 核心表（temple/master/service_type/extra_service/material/user/booking/message/merit_money_tier）
-- 业务域分库：askxuan_auth/askxuan_user/askxuan_temple/askxuan_master/askxuan_booking/askxuan_message/askxuan_shop/askxuan_diy/askxuan_finance/askxuan_review/askxuan_audit/askxuan_logistics/askxuan_marketing/askxuan_ai/askxuan_media/askxuan_community/askxuan_system
-- askxuan_message 库：3 张表（message_template/push_log/system_announcement）
-- 验证：
--   SELECT code,name,sect FROM askxuan.temple;            -- 应返回 6 行
--   SELECT code,dharma_name,sect FROM askxuan.master;     -- 应返回 6 行
--   SELECT code,name,price FROM askxuan.extra_service;    -- 应返回 4 行，价格 168/128/198/268
--   SELECT id,name,code FROM askxuan_auth.role;           -- 应返回 5 行
--   SELECT word,category FROM askxuan_audit.sensitive_word; -- 应返回 5 行
--   SELECT code,name FROM askxuan_ai.ai_skill;            -- 应返回 7 行
-- ============================================================
