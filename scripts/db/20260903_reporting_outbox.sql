-- 权威经营报表与 MQ 可靠投递前向迁移（可重复执行）

CREATE TABLE IF NOT EXISTS askxuan_booking.event_outbox (
  id BIGINT NOT NULL AUTO_INCREMENT,
  event_key VARCHAR(160) NOT NULL,
  aggregate_type VARCHAR(48) NOT NULL,
  aggregate_id VARCHAR(96) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  exchange_name VARCHAR(64) NOT NULL,
  routing_key VARCHAR(96) NOT NULL DEFAULT '',
  payload JSON NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/processing/sent/dead',
  retry_count INT NOT NULL DEFAULT 0,
  next_retry_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  locked_at DATETIME NULL,
  last_error VARCHAR(500) NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  sent_at DATETIME NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_event_key (event_key),
  KEY idx_outbox_due (status,next_retry_at),
  KEY idx_outbox_aggregate (aggregate_type,aggregate_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='预约域事务发件箱';

CREATE TABLE IF NOT EXISTS askxuan_payment.event_outbox LIKE askxuan_booking.event_outbox;
CREATE TABLE IF NOT EXISTS askxuan_diy.event_outbox LIKE askxuan_booking.event_outbox;

-- 旧数据由各服务补偿扫描器按稳定 event_key 自动回填，不在迁移中跨库伪造业务状态。
