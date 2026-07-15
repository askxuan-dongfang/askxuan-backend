-- Repair deterministic demo rows created before service databases shared canonical IDs.
-- Safe to run repeatedly: every update is scoped by a stable seed business key.
SET NAMES utf8mb4;
USE askxuan_temple;
START TRANSACTION;

INSERT IGNORE INTO askxuan_auth.admin_account
  (id,account,password,name,role_id,temple_id,master_id,status,create_time)
VALUES
  (4,'baiyun_admin','123456','白云观管理员',2,'T002','','enabled','2026-07-01 00:00:00');

UPDATE askxuan_temple.temple_admin SET account_id=2 WHERE temple_code='T001' AND account_id=1001;
UPDATE askxuan_temple.temple_admin SET account_id=4 WHERE temple_code='T002' AND account_id=1002;

UPDATE askxuan_booking.booking SET user_id=1 WHERE booking_no IN ('B20260630001','B20260615003') AND user_id=1001;
UPDATE askxuan_booking.booking SET user_id=2 WHERE booking_no='B20260628002' AND user_id=1002;
UPDATE askxuan_booking.booking_status_log SET operator_id='1' WHERE booking_id='B20260630001' AND operator_id='1001';
UPDATE askxuan_booking.booking_review SET user_id='1' WHERE booking_id='B20260615003' AND user_id='1001';

UPDATE askxuan_message.message SET user_id='1' WHERE biz_id IN ('B20260630001','B20260615003') AND user_id='1001';
UPDATE askxuan_message.message SET user_id='2' WHERE biz_id='B20260628002' AND user_id='1002';
UPDATE askxuan_user.user_address SET user_id=1 WHERE phone='13800138000' AND user_id=1001;
UPDATE askxuan_user.user_profile legacy
LEFT JOIN askxuan_user.user_profile canonical ON canonical.user_id=1
SET legacy.user_id=1
WHERE legacy.user_id=1001 AND canonical.user_id IS NULL;

UPDATE askxuan_diy.diy_design SET user_id='1' WHERE design_no='DD20260628001' AND user_id='1001';
UPDATE askxuan_diy.diy_order SET user_id='1',material_fee=348.00,total_fee=516.00 WHERE order_no='DIY20260630001' AND user_id='1001';
UPDATE askxuan_order.shop_order SET user_id='1' WHERE order_no='SO20260620001' AND user_id='1001';
UPDATE askxuan_payment.payment SET user_id='1' WHERE payment_no='PAY20260620001' AND user_id='1001';
UPDATE askxuan_ai.ai_session SET user_id='1' WHERE session_no='AI20260630001' AND user_id='1001';

UPDATE askxuan_review.review SET user_id='1' WHERE review_no IN ('RV20260620001','RV20260628003') AND user_id='1001';
UPDATE askxuan_review.review SET user_id='2' WHERE review_no='RV20260625002' AND user_id='1002';
UPDATE askxuan_audit.audit_queue SET submitter_id='1' WHERE biz_id='DD20260628001' AND submitter_id='1001';
UPDATE askxuan_audit.audit_queue SET submitter_id='2' WHERE biz_id='DD20260620002' AND submitter_id='1002';
UPDATE askxuan_audit.report SET reporter_id='1' WHERE target_id='RV20260610005' AND reporter_id='1001';
UPDATE askxuan_audit.report SET reporter_id='2' WHERE target_id='DD20260615001' AND reporter_id='1002';
UPDATE askxuan_marketing.coupon_record SET user_id='1' WHERE coupon_no='C20260700001' AND user_id='1001';

-- Older forward migrations created the table but only copied a partial temple catalog.
-- INSERT IGNORE preserves prices and shelf states already maintained by operators.
INSERT IGNORE INTO askxuan_temple.temple_service_intent_tag (temple_service_id,tag_code)
SELECT MIN(s.id), tags.tag_code
FROM askxuan_temple.temple_service s
JOIN askxuan_temple.temple_service_intent_tag tags ON tags.temple_service_id=s.id
GROUP BY s.temple_code,s.service_code,tags.tag_code;

DELETE duplicate
FROM askxuan_temple.temple_service duplicate
JOIN askxuan_temple.temple_service keeper
  ON keeper.temple_code=duplicate.temple_code
 AND keeper.service_code=duplicate.service_code
 AND keeper.id<duplicate.id;

DELETE tags
FROM askxuan_temple.temple_service_intent_tag tags
LEFT JOIN askxuan_temple.temple_service service ON service.id=tags.temple_service_id
WHERE service.id IS NULL;

SET @temple_service_unique_sql = IF(
  EXISTS(
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema='askxuan_temple' AND table_name='temple_service' AND index_name='uk_temple_service'
  ),
  'DO 1',
  'ALTER TABLE askxuan_temple.temple_service ADD UNIQUE KEY uk_temple_service (temple_code,service_code)'
);
PREPARE temple_service_unique_stmt FROM @temple_service_unique_sql;
EXECUTE temple_service_unique_stmt;
DEALLOCATE PREPARE temple_service_unique_stmt;

INSERT IGNORE INTO askxuan_temple.temple_service
  (temple_code,service_code,service_name,price,time_slots,status,create_time)
VALUES
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

INSERT IGNORE INTO askxuan_temple.temple_service_intent_tag (temple_service_id,tag_code)
SELECT id, CASE service_code
  WHEN 'S001' THEN 'peace' WHEN 'S002' THEN 'love' WHEN 'S003' THEN 'wealth'
  WHEN 'S005' THEN 'rite' WHEN 'S006' THEN 'career' WHEN 'S007' THEN 'taisui'
  WHEN 'S008' THEN 'love' WHEN 'S009' THEN 'wealth' WHEN 'S010' THEN 'career'
  WHEN 'S011' THEN 'career' WHEN 'S012' THEN 'peace' WHEN 'S013' THEN 'study'
END
FROM askxuan_temple.temple_service
WHERE service_code IN ('S001','S002','S003','S005','S006','S007','S008','S009','S010','S011','S012','S013');

COMMIT;
