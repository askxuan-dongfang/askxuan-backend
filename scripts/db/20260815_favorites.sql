SET NAMES utf8mb4;

-- ============================================================
-- 寺院收藏 + 商品收藏（幂等迁移）
-- ============================================================

USE askxuan_temple;
CREATE TABLE IF NOT EXISTS `temple_favorite` (
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `temple_code` VARCHAR(16) NOT NULL COMMENT '寺院编码',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`,`temple_code`),
  KEY `idx_user` (`user_id`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户收藏寺院';

USE askxuan_product;
CREATE TABLE IF NOT EXISTS `product_favorite` (
  `user_id` BIGINT NOT NULL COMMENT '用户ID',
  `product_id` BIGINT NOT NULL COMMENT '商品ID',
  `create_time` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`user_id`,`product_id`),
  KEY `idx_user` (`user_id`,`create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户收藏商品';
