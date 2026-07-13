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

-- ============ 2. 拆分 askxuan_shop 共享库 ============
-- payment 表 → askxuan_payment
CREATE DATABASE IF NOT EXISTS askxuan_payment CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- product 表 → askxuan_product
CREATE DATABASE IF NOT EXISTS askxuan_product CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- order 表 → askxuan_order
CREATE DATABASE IF NOT EXISTS askxuan_order CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 迁移 payment 相关表（如果 askxuan_shop 中存在）
-- 注意：RENAME TABLE 跨库迁移会丢失外键约束，本项目无外键约束
INSERT IGNORE INTO askxuan_payment.payment SELECT * FROM askxuan_shop.payment WHERE 1=0;
INSERT IGNORE INTO askxuan_payment.payment_log SELECT * FROM askxuan_shop.payment_log WHERE 1=0;
INSERT IGNORE INTO askxuan_payment.refund SELECT * FROM askxuan_shop.refund WHERE 1=0;

-- 实际迁移数据（取消 WHERE 1=0）
-- 注意：执行前请确认 askxuan_payment 等库中已创建对应表结构
-- 如果表结构未创建，需要先执行 init.sql 中的对应部分

-- ============ 3. 授权各账户访问自己的库 ============

GRANT ALL PRIVILEGES ON askxuan_auth.* TO 'auth_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_user.* TO 'user_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_temple.* TO 'temple_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_master.* TO 'master_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_booking.* TO 'booking_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_review.* TO 'review_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_product.* TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service_intent_tag TO 'product_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_diy.* TO 'diy_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_order.* TO 'order_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_payment.* TO 'payment_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_finance.* TO 'finance_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_audit.* TO 'audit_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_message.* TO 'message_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_logistics.* TO 'logistics_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_marketing.* TO 'marketing_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_file.* TO 'file_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_ai.* TO 'ai_user'@'%';

FLUSH PRIVILEGES;

-- ============ 4. 收紧 root 权限（可选，生产环境建议） ============
-- REVOKE ALL PRIVILEGES ON *.* FROM 'root'@'%';
-- GRANT ALL PRIVILEGES ON *.* TO 'root'@'localhost' WITH GRANT OPTION;
-- 注意：收紧 root 权限前请确认所有服务已切换到独立账户
