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
CREATE DATABASE IF NOT EXISTS askxuan_media CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_community CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 将仍在默认/共享库的核心表复制到服务实际使用的数据库。
CREATE TABLE IF NOT EXISTS askxuan_user.user LIKE askxuan.user;
INSERT IGNORE INTO askxuan_user.user SELECT * FROM askxuan.user;
CREATE TABLE IF NOT EXISTS askxuan_temple.temple LIKE askxuan.temple;
INSERT IGNORE INTO askxuan_temple.temple (id,code,name,region,type,sect,status,address,cover_image,rating,description,create_time,update_time)
SELECT id,code,name,region,type,sect,status,address,cover_image,rating,description,create_time,update_time FROM askxuan.temple;
CREATE TABLE IF NOT EXISTS askxuan_temple.service_type LIKE askxuan.service_type;
INSERT IGNORE INTO askxuan_temple.service_type SELECT * FROM askxuan.service_type;
CREATE TABLE IF NOT EXISTS askxuan_master.master LIKE askxuan.master;
INSERT IGNORE INTO askxuan_master.master (id,code,dharma_name,lay_name,temple_code,position,sect,type,auth_status,shelf_status,platform_status,specialties,avatar,rating,create_time,update_time)
SELECT id,code,dharma_name,lay_name,temple_code,position,sect,type,auth_status,shelf_status,platform_status,specialties,avatar,rating,create_time,update_time FROM askxuan.master;
CREATE TABLE IF NOT EXISTS askxuan_booking.booking LIKE askxuan.booking;
INSERT IGNORE INTO askxuan_booking.booking SELECT * FROM askxuan.booking;
CREATE TABLE IF NOT EXISTS askxuan_message.message LIKE askxuan.message;
INSERT IGNORE INTO askxuan_message.message SELECT * FROM askxuan.message;

CREATE TABLE IF NOT EXISTS askxuan_order.shop_order LIKE askxuan_shop.shop_order;
CREATE TABLE IF NOT EXISTS askxuan_order.shop_order_item LIKE askxuan_shop.shop_order_item;
CREATE TABLE IF NOT EXISTS askxuan_order.shop_order_logistics LIKE askxuan_shop.shop_order_logistics;
CREATE TABLE IF NOT EXISTS askxuan_order.return_order LIKE askxuan_shop.return_order;
INSERT IGNORE INTO askxuan_order.shop_order SELECT * FROM askxuan_shop.shop_order;
INSERT IGNORE INTO askxuan_order.shop_order_item SELECT * FROM askxuan_shop.shop_order_item;
INSERT IGNORE INTO askxuan_order.shop_order_logistics SELECT * FROM askxuan_shop.shop_order_logistics;
INSERT IGNORE INTO askxuan_order.return_order SELECT * FROM askxuan_shop.return_order;
CREATE TABLE IF NOT EXISTS askxuan_payment.payment LIKE askxuan_shop.payment;
CREATE TABLE IF NOT EXISTS askxuan_payment.payment_log LIKE askxuan_shop.payment_log;
CREATE TABLE IF NOT EXISTS askxuan_payment.refund LIKE askxuan_shop.refund;

-- 迁移 payment 相关表；主键/唯一键保证脚本可重复执行。
INSERT IGNORE INTO askxuan_payment.payment SELECT * FROM askxuan_shop.payment;
INSERT IGNORE INTO askxuan_payment.payment_log SELECT * FROM askxuan_shop.payment_log;
INSERT IGNORE INTO askxuan_payment.refund SELECT * FROM askxuan_shop.refund;

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
GRANT SELECT ON askxuan_temple.temple TO 'booking_user'@'%';
GRANT SELECT ON askxuan_temple.service_type TO 'booking_user'@'%';
GRANT SELECT ON askxuan_master.master TO 'booking_user'@'%';
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
