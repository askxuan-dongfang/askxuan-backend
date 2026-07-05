-- ============================================================
-- Outbox 表（事务性发件箱，用于 order-service 异步消息可靠投递）
-- 阶段 5：order→payment 退款异步化 + Outbox 模式改造
-- 执行方式：mysql -uroot -proot123 < outbox.sql
-- ============================================================

-- outbox 表必须与 return_order 表位于同一数据库（事务原子性要求）。
-- order-service 数据源连接 askxuan_order 库（阶段 4 拆分后），
-- outbox 表与 return_order/shop_order 同库。
USE `askxuan_order`;

CREATE TABLE IF NOT EXISTS `outbox` (
    `id` BIGINT NOT NULL AUTO_INCREMENT,
    `aggregate_id` VARCHAR(64) NOT NULL COMMENT '业务聚合 ID（如退货单号 return_no）',
    `message_type` VARCHAR(64) NOT NULL COMMENT '消息类型（如 refund.request）',
    `payload` TEXT NOT NULL COMMENT 'JSON 消息体',
    `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/sent/failed',
    `retry_count` INT NOT NULL DEFAULT 0 COMMENT '重试次数（达到 5 次后标记为 failed）',
    `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    INDEX `idx_status` (`status`),
    INDEX `idx_aggregate` (`aggregate_id`),
    INDEX `idx_created` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事务性发件箱（order-service 异步消息投递）';

-- 验证：
--   DESC askxuan_shop.outbox;
--   SELECT status, COUNT(*) FROM askxuan_shop.outbox GROUP BY status;
