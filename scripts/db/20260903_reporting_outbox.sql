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

-- 首次启用 outbox 时把既有业务事实登记为已发送基线，避免历史通知被重新广播。
-- INSERT IGNORE 不会覆盖部署期间已经由业务事务写入的 pending 事件。
INSERT IGNORE INTO askxuan_booking.event_outbox
  (event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at,sent_at)
SELECT CONCAT('booking:',l.booking_id,':',IF(l.to_status='pending','created',l.to_status)),
       'booking',l.booking_id,CONCAT('booking.',IF(l.to_status='pending','created',l.to_status)),
       'booking.events','',JSON_OBJECT('baseline',TRUE),'sent',0,l.create_time,l.create_time,l.create_time,l.create_time
FROM askxuan_booking.booking_status_log l
WHERE l.to_status<>'pending_payment';

INSERT IGNORE INTO askxuan_booking.event_outbox
  (event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at,sent_at)
SELECT CONCAT('consultation:',c.order_no,':paid'),'consultation',c.order_no,'consultation.paid',
       'consultation.events','',JSON_OBJECT('baseline',TRUE),'sent',0,c.valid_from,c.valid_from,c.valid_from,c.valid_from
FROM askxuan_booking.consultation_order c
WHERE c.payment_status='success' AND c.valid_from IS NOT NULL;

INSERT IGNORE INTO askxuan_payment.event_outbox
  (event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at,sent_at)
SELECT CONCAT('payment:',p.payment_no,':',p.status),'payment',p.payment_no,CONCAT('payment.',p.status),
       'payment.events','',JSON_OBJECT('baseline',TRUE),'sent',0,p.update_time,p.create_time,p.update_time,p.update_time
FROM askxuan_payment.payment p
WHERE p.status IN ('success','failed','refunded');

INSERT IGNORE INTO askxuan_diy.event_outbox
  (event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at,sent_at)
SELECT CONCAT('diy:blessing:',t.task_no,':dispatch'),'diy_blessing',t.task_no,'blessing.dispatch',
       'blessing.events','',JSON_OBJECT('baseline',TRUE),'sent',0,t.create_time,t.create_time,t.update_time,t.update_time
FROM askxuan_diy.blessing_task t
WHERE t.status='dispatched';

INSERT IGNORE INTO askxuan_diy.event_outbox
  (event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at,sent_at)
SELECT CONCAT('diy:order:',o.order_no,':shipped'),'diy_order',o.order_no,'order.shipped',
       'order.events','',JSON_OBJECT('baseline',TRUE),'sent',0,o.update_time,o.create_time,o.update_time,o.update_time
FROM askxuan_diy.diy_order o
WHERE o.status='shipped';
