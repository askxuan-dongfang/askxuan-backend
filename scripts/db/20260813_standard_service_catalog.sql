-- 寺院服务必须引用平台固定 service_type，名称不再由寺院自定义。
SET NAMES utf8mb4;

INSERT INTO askxuan_temple.service_type
  (code,name,type,price_range,master_codes)
VALUES
  ('S001','祈福','法事','¥100-500','M001,M002,M004,M005'),
  ('S002','供灯','供养','¥50-300','M001,M003,M004'),
  ('S003','上香','供养','¥30-200','M001,M003,M004'),
  ('S004','还愿','法事','¥100-500','M001,M003,M004'),
  ('S005','超度','法事','¥300-1000','M001,M003,M004'),
  ('S006','开光','法事','¥168-500','M001,M003'),
  ('S007','化太岁','法事','¥200-800','M002,M006'),
  ('S008','求姻缘','祈愿','¥100-500','M001,M005'),
  ('S009','求财运','祈愿','¥100-800','M002,M006'),
  ('S010','求事业','祈愿','¥100-600','M003,M006'),
  ('S011','求风水','咨询','¥200-1000','M002,M006'),
  ('S012','求健康','祈愿','¥100-600','M001,M004'),
  ('S013','求学业','祈愿','¥100-500','M003,M005')
ON DUPLICATE KEY UPDATE
  name=VALUES(name), type=VALUES(type), price_range=VALUES(price_range), master_codes=VALUES(master_codes);

UPDATE askxuan_temple.temple_service AS temple_service
JOIN askxuan_temple.service_type AS service_type
  ON service_type.code = temple_service.service_code
SET temple_service.service_name = service_type.name
WHERE temple_service.service_name <> service_type.name;
