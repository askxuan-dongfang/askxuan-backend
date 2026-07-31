USE askxuan_diy;

SET @extra_service_status_exists = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema='askxuan_diy' AND table_name='extra_service' AND column_name='status'
);
SET @extra_service_status_sql = IF(
  @extra_service_status_exists = 0,
  'ALTER TABLE askxuan_diy.extra_service ADD COLUMN status VARCHAR(32) NOT NULL DEFAULT ''on_shelf'' AFTER description',
  'SELECT 1'
);
PREPARE extra_service_status_stmt FROM @extra_service_status_sql;
EXECUTE extra_service_status_stmt;
DEALLOCATE PREPARE extra_service_status_stmt;

DELETE duplicate_task FROM askxuan_diy.blessing_task duplicate_task
JOIN askxuan_diy.blessing_task retained
  ON duplicate_task.diy_order_no=retained.diy_order_no AND duplicate_task.id>retained.id;
SET @blessing_order_unique_exists = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema='askxuan_diy' AND table_name='blessing_task' AND index_name='uk_diy_order_no'
);
SET @blessing_order_unique_sql = IF(
  @blessing_order_unique_exists = 0,
  'ALTER TABLE askxuan_diy.blessing_task ADD UNIQUE KEY uk_diy_order_no(diy_order_no)',
  'SELECT 1'
);
PREPARE blessing_order_unique_stmt FROM @blessing_order_unique_sql;
EXECUTE blessing_order_unique_stmt;
DEALLOCATE PREPARE blessing_order_unique_stmt;

DELETE cr1 FROM askxuan_marketing.coupon_record cr1
JOIN askxuan_marketing.coupon_record cr2
  ON cr1.coupon_id=cr2.coupon_id AND cr1.user_id=cr2.user_id AND cr1.id>cr2.id;

SET @coupon_user_index_exists = (
  SELECT COUNT(1)
  FROM information_schema.statistics
  WHERE table_schema = 'askxuan_marketing'
    AND table_name = 'coupon_record'
    AND index_name = 'uk_coupon_user'
);
SET @coupon_user_index_sql = IF(
  @coupon_user_index_exists = 0,
  'ALTER TABLE askxuan_marketing.coupon_record ADD UNIQUE KEY uk_coupon_user (coupon_id, user_id)',
  'SELECT 1'
);
PREPARE coupon_user_index_stmt FROM @coupon_user_index_sql;
EXECUTE coupon_user_index_stmt;
DEALLOCATE PREPARE coupon_user_index_stmt;

DELETE aq1 FROM askxuan_audit.audit_queue aq1
JOIN askxuan_audit.audit_queue aq2
  ON aq1.biz_type=aq2.biz_type AND aq1.biz_id=aq2.biz_id AND aq1.id>aq2.id;

SET @audit_biz_index_exists = (
  SELECT COUNT(1)
  FROM information_schema.statistics
  WHERE table_schema = 'askxuan_audit'
    AND table_name = 'audit_queue'
    AND index_name = 'uk_biz_type_id'
);
SET @audit_biz_index_sql = IF(
  @audit_biz_index_exists = 0,
  'ALTER TABLE askxuan_audit.audit_queue ADD UNIQUE KEY uk_biz_type_id (biz_type, biz_id)',
  'SELECT 1'
);
PREPARE audit_biz_index_stmt FROM @audit_biz_index_sql;
EXECUTE audit_biz_index_stmt;
DEALLOCATE PREPARE audit_biz_index_stmt;

CREATE TABLE IF NOT EXISTS askxuan_master.master_earning (
  id BIGINT NOT NULL AUTO_INCREMENT,
  source_type VARCHAR(32) NOT NULL,
  source_id VARCHAR(64) NOT NULL,
  master_code VARCHAR(16) NOT NULL,
  earning_date DATE NOT NULL,
  service_type VARCHAR(32) NOT NULL,
  service_name VARCHAR(128) NOT NULL DEFAULT '',
  user_name VARCHAR(64) NOT NULL DEFAULT '',
  amount DECIMAL(12,2) NOT NULL DEFAULT 0,
  settle_status VARCHAR(32) NOT NULL DEFAULT 'pending',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_source (source_type,source_id),
  KEY idx_master_date (master_code,earning_date),
  KEY idx_master_settle (master_code,settle_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师收益明细';

INSERT IGNORE INTO askxuan_master.master_earning
(source_type,source_id,master_code,earning_date,service_type,service_name,user_name,amount,settle_status) VALUES
('booking','seed-booking-001','M001','2026-07-01','booking','祈福法会','U001',500.00,'settled'),
('diy_blessing','seed-blessing-001','M001','2026-07-01','diy_blessing','开光加持','U002',300.00,'pending'),
('consult','seed-consult-001','M001','2026-06-28','consult','线上咨询','U003',200.00,'withdrew');

CREATE TABLE IF NOT EXISTS askxuan_master.master_profile_ext (
  master_code VARCHAR(16) NOT NULL,
  bio VARCHAR(512) NOT NULL DEFAULT '',
  pricing VARCHAR(512) NOT NULL DEFAULT '',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (master_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='法师工作台资料扩展';

INSERT INTO askxuan_master.master_profile_ext (master_code,bio,pricing) VALUES
('M001','普陀山出家，擅长祈福法事与超度仪轨，弘法二十余载。','预约法事 200-800 元 / DIY加持 300-500 元'),
('M002','武当山修道，精通道教科仪与养生功法。','道教科仪 500-1200 元 / 养生咨询 200 元')
ON DUPLICATE KEY UPDATE master_code=VALUES(master_code);

SET @review_master_column_exists = (
  SELECT COUNT(1) FROM information_schema.columns
  WHERE table_schema='askxuan_review' AND table_name='review' AND column_name='master_code'
);
SET @review_master_column_sql = IF(
  @review_master_column_exists = 0,
  'ALTER TABLE askxuan_review.review ADD COLUMN master_code VARCHAR(16) NOT NULL DEFAULT '''' AFTER target_id',
  'SELECT 1'
);
PREPARE review_master_column_stmt FROM @review_master_column_sql;
EXECUTE review_master_column_stmt;
DEALLOCATE PREPARE review_master_column_stmt;

UPDATE askxuan_review.review r
JOIN askxuan_booking.booking b ON r.target_type='booking' AND r.target_id=b.booking_no
SET r.master_code=b.master_code
WHERE r.master_code='';

DELETE r1 FROM askxuan_review.review r1
JOIN askxuan_review.review r2
  ON r1.user_id=r2.user_id AND r1.target_type=r2.target_type AND r1.target_id=r2.target_id AND r1.id>r2.id;

SET @review_target_index_exists = (
  SELECT COUNT(1) FROM information_schema.statistics
  WHERE table_schema='askxuan_review' AND table_name='review' AND index_name='uk_user_target'
);
SET @review_target_index_sql = IF(
  @review_target_index_exists = 0,
  'ALTER TABLE askxuan_review.review ADD UNIQUE KEY uk_user_target (user_id,target_type,target_id)',
  'SELECT 1'
);
PREPARE review_target_index_stmt FROM @review_target_index_sql;
EXECUTE review_target_index_stmt;
DEALLOCATE PREPARE review_target_index_stmt;
