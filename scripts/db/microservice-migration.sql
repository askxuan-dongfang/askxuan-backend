-- 问玄东方微服务化改造 - MySQL 数据迁移脚本
-- 阶段 4：拆分共享库 + 创建独立账户
-- 执行方式：mysql -uroot -proot123 < microservice-migration.sql

-- ============ 1. 创建独立服务账户 ============
-- 每个账户只能访问自己的数据库

CREATE USER IF NOT EXISTS 'auth_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'user_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'temple_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'master_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'booking_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'review_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'product_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'diy_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'order_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'payment_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'finance_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'audit_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'message_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'logistics_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'marketing_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'file_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'ai_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'media_user'@'%' IDENTIFIED BY 'Askxuan2026!';
CREATE USER IF NOT EXISTS 'community_user'@'%' IDENTIFIED BY 'Askxuan2026!';

-- ============ 2. 拆分 askxuan_shop 共享库 ============
-- payment 表 → askxuan_payment
CREATE DATABASE IF NOT EXISTS askxuan_payment CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- product 表 → askxuan_product
CREATE DATABASE IF NOT EXISTS askxuan_product CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- order 表 → askxuan_order
CREATE DATABASE IF NOT EXISTS askxuan_order CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_user CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_temple CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_master CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_booking CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_message CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_media CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_community CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_system CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE askxuan_system;

-- 源共享表存在时才复制；完成分库且已删除旧共享表时安全跳过。
DROP PROCEDURE IF EXISTS migrate_table_if_present;
DELIMITER //
CREATE PROCEDURE migrate_table_if_present(IN source_schema VARCHAR(64), IN target_schema VARCHAR(64), IN target_table VARCHAR(64))
BEGIN
  IF EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema=source_schema AND table_name=target_table) THEN
    SET @sql = CONCAT('CREATE TABLE IF NOT EXISTS `', target_schema, '`.`', target_table, '` LIKE `', source_schema, '`.`', target_table, '`');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @sql = CONCAT('INSERT IGNORE INTO `', target_schema, '`.`', target_table, '` SELECT * FROM `', source_schema, '`.`', target_table, '`');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
  END IF;
END//
DELIMITER ;

CALL migrate_table_if_present('askxuan', 'askxuan_user', 'user');
CALL migrate_table_if_present('askxuan', 'askxuan_temple', 'service_type');
CALL migrate_table_if_present('askxuan', 'askxuan_booking', 'booking');
CALL migrate_table_if_present('askxuan', 'askxuan_message', 'message');
CALL migrate_table_if_present('askxuan_shop', 'askxuan_order', 'shop_order');
CALL migrate_table_if_present('askxuan_shop', 'askxuan_order', 'shop_order_item');
CALL migrate_table_if_present('askxuan_shop', 'askxuan_order', 'shop_order_logistics');
CALL migrate_table_if_present('askxuan_shop', 'askxuan_order', 'return_order');
CALL migrate_table_if_present('askxuan_shop', 'askxuan_payment', 'payment');
CALL migrate_table_if_present('askxuan_shop', 'askxuan_payment', 'payment_log');
CALL migrate_table_if_present('askxuan_shop', 'askxuan_payment', 'refund');
DROP PROCEDURE migrate_table_if_present;

-- ============ 3. 授权各账户访问自己的库 ============

GRANT ALL PRIVILEGES ON askxuan_auth.* TO 'auth_user'@'%';
GRANT SELECT ON askxuan_user.user TO 'auth_user'@'%';
GRANT SELECT ON askxuan_temple.temple TO 'auth_user'@'%';
GRANT SELECT ON askxuan_master.master TO 'auth_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_user.* TO 'user_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_temple.* TO 'temple_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_master.* TO 'master_user'@'%';
GRANT SELECT ON askxuan_temple.temple TO 'master_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_booking.* TO 'booking_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_review.* TO 'review_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_product.* TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service_intent_tag TO 'product_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_diy.* TO 'diy_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_order.* TO 'order_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_payment.* TO 'payment_user'@'%';
GRANT SELECT ON askxuan_diy.diy_order TO 'payment_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_finance.* TO 'finance_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_audit.* TO 'audit_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_message.* TO 'message_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_logistics.* TO 'logistics_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_marketing.* TO 'marketing_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_file.* TO 'file_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_ai.* TO 'ai_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_media.* TO 'media_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_community.* TO 'community_user'@'%';
GRANT SELECT ON askxuan_media.media_asset TO 'community_user'@'%';
GRANT SELECT,INSERT,UPDATE ON askxuan_audit.audit_queue TO 'community_user'@'%';
GRANT SELECT,INSERT ON askxuan_audit.audit_log TO 'community_user'@'%';

FLUSH PRIVILEGES;

-- ============ 4. 收紧 root 权限（可选，生产环境建议） ============
-- REVOKE ALL PRIVILEGES ON *.* FROM 'root'@'%';
-- GRANT ALL PRIVILEGES ON *.* TO 'root'@'localhost' WITH GRANT OPTION;
-- 注意：收紧 root 权限前请确认所有服务已切换到独立账户
