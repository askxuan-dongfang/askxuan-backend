SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS askxuan_temple.temple_service_slot (
  id BIGINT NOT NULL AUTO_INCREMENT,
  temple_service_id BIGINT NOT NULL,
  slot_code VARCHAR(32) NOT NULL,
  label VARCHAR(64) NOT NULL,
  start_time VARCHAR(5) NOT NULL,
  end_time VARCHAR(5) NOT NULL,
  capacity INT NOT NULL DEFAULT 10,
  status VARCHAR(16) NOT NULL DEFAULT 'enabled',
  sort INT NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_service_slot (temple_service_id, slot_code),
  KEY idx_service_status (temple_service_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='寺院服务结构化预约时段';

INSERT IGNORE INTO askxuan_temple.temple_service_slot
  (temple_service_id,slot_code,label,start_time,end_time,capacity,status,sort)
SELECT s.id,
       CONCAT('slot_', LPAD(j.ord, 2, '0')),
       CONCAT('时段', j.ord),
       SUBSTRING_INDEX(j.slot_range, '-', 1),
       SUBSTRING_INDEX(j.slot_range, '-', -1),
       10,
       'enabled',
       j.ord
FROM askxuan_temple.temple_service s
JOIN JSON_TABLE(
  IF(JSON_VALID(s.time_slots), s.time_slots, JSON_ARRAY()),
  '$[*]' COLUMNS(ord FOR ORDINALITY, slot_range VARCHAR(32) PATH '$')
) j
WHERE j.slot_range LIKE '%-%';

CREATE TABLE IF NOT EXISTS askxuan_booking.booking_slot_inventory (
  id BIGINT NOT NULL AUTO_INCREMENT,
  temple_code VARCHAR(16) NOT NULL,
  service_code VARCHAR(16) NOT NULL,
  booking_date DATE NOT NULL,
  slot_code VARCHAR(32) NOT NULL,
  time_slot VARCHAR(32) NOT NULL,
  capacity INT NOT NULL,
  reserved_count INT NOT NULL DEFAULT 0,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uk_booking_slot (temple_code,service_code,booking_date,slot_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='预约日期时段库存';

SET @booking_schema='askxuan_booking';
SET @booking_table='booking';

SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='request_id'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN request_id VARCHAR(64) NULL AFTER booking_no'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='temple_name'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN temple_name VARCHAR(128) NOT NULL DEFAULT "" AFTER temple_code'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='master_name'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN master_name VARCHAR(64) NOT NULL DEFAULT "" AFTER master_code'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='service_name'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN service_name VARCHAR(128) NOT NULL DEFAULT "" AFTER service_code'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='slot_code'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN slot_code VARCHAR(32) NOT NULL DEFAULT "" AFTER booking_date'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='service_fee'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN service_fee DECIMAL(10,2) NOT NULL DEFAULT 0 AFTER time_slot'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='total_fee'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN total_fee DECIMAL(10,2) NOT NULL DEFAULT 0 AFTER merit_money_tier'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='price_snapshot'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN price_snapshot JSON NULL AFTER total_fee'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='payment_no'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN payment_no VARCHAR(32) NOT NULL DEFAULT "" AFTER price_snapshot'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='payment_channel'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN payment_channel VARCHAR(16) NOT NULL DEFAULT "" AFTER payment_no'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='payment_status'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN payment_status VARCHAR(16) NOT NULL DEFAULT "legacy" AFTER payment_channel'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='payment_expire_time'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN payment_expire_time DATETIME NULL AFTER payment_status'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@booking_schema AND table_name=@booking_table AND column_name='slot_reserved'),'DO 1','ALTER TABLE askxuan_booking.booking ADD COLUMN slot_reserved TINYINT NOT NULL DEFAULT 0 AFTER payment_expire_time'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

UPDATE askxuan_booking.booking b
LEFT JOIN askxuan_temple.temple t ON t.code=b.temple_code
LEFT JOIN askxuan_master.master m ON m.code=b.master_code
LEFT JOIN askxuan_temple.temple_service s ON s.temple_code=b.temple_code AND s.service_code=b.service_code
SET b.temple_name=COALESCE(NULLIF(b.temple_name,''),t.name,''),
    b.master_name=COALESCE(NULLIF(b.master_name,''),m.dharma_name,''),
    b.service_name=COALESCE(NULLIF(b.service_name,''),s.service_name,''),
    b.total_fee=IF(b.total_fee=0,b.merit_money,b.total_fee),
    b.price_snapshot=COALESCE(b.price_snapshot,JSON_OBJECT('serviceFee',0,'meritMoney',b.merit_money,'totalFee',b.merit_money,'legacy',TRUE));

SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=@booking_schema AND table_name=@booking_table AND index_name='uk_booking_request'),'DO 1','ALTER TABLE askxuan_booking.booking ADD UNIQUE KEY uk_booking_request (user_id,request_id)'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=@booking_schema AND table_name=@booking_table AND index_name='idx_payment_status'),'DO 1','ALTER TABLE askxuan_booking.booking ADD KEY idx_payment_status (payment_status,payment_expire_time)'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;

SET @payment_schema='askxuan_payment';
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=@payment_schema AND table_name='payment' AND column_name='idempotency_key'),'DO 1','ALTER TABLE askxuan_payment.payment ADD COLUMN idempotency_key VARCHAR(96) NULL AFTER payment_no'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
SET @sql=IF(EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema=@payment_schema AND table_name='payment' AND index_name='uk_payment_idempotency'),'DO 1','ALTER TABLE askxuan_payment.payment ADD UNIQUE KEY uk_payment_idempotency (idempotency_key)'); PREPARE s FROM @sql; EXECUTE s; DEALLOCATE PREPARE s;
