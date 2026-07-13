-- Requirement 6: intention tags and cross-domain aggregation. Safe to run repeatedly as root.
CREATE DATABASE IF NOT EXISTS askxuan_product DEFAULT CHARACTER SET utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_product.intent_tag (
  code VARCHAR(32) NOT NULL PRIMARY KEY, name VARCHAR(64) NOT NULL,
  description VARCHAR(255) NOT NULL DEFAULT '', icon VARCHAR(64) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0, status VARCHAR(16) NOT NULL DEFAULT 'enabled',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  KEY idx_intent_status_sort(status, sort)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_product.product_intent_tag (
  product_id BIGINT NOT NULL, tag_code VARCHAR(32) NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(product_id, tag_code), KEY idx_product_intent_code(tag_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_temple.temple_service_intent_tag (
  temple_service_id BIGINT NOT NULL, tag_code VARCHAR(32) NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(temple_service_id, tag_code), KEY idx_intent_tag_code(tag_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT INTO askxuan_product.intent_tag(code,name,description,icon,sort) VALUES
('peace','求平安','祈福、护佑与健康相关商品和服务','shield.lefthalf.filled',10),
('wealth','求财运','财运、供养与事业助力相关商品和服务','banknote.fill',20),
('love','求姻缘','姻缘、人际与家庭相关商品和服务','heart.fill',30),
('career','求事业','事业、风水与开光相关商品和服务','briefcase.fill',40),
('study','求学业','学业、智慧与考试相关商品和服务','book.fill',50),
('taisui','化太岁','本命年与化太岁相关服务','circle.hexagongrid.fill',60),
('diy','定手串','手串材料与定制相关商品','circle.grid.cross.fill',70),
('rite','做法事','超度等法事服务','hands.sparkles.fill',80)
ON DUPLICATE KEY UPDATE name=VALUES(name),description=VALUES(description),icon=VALUES(icon),sort=VALUES(sort);

INSERT IGNORE INTO askxuan_product.product_intent_tag(product_id,tag_code)
SELECT id,'diy' FROM askxuan_product.product WHERE product_no IN ('P20260600001','P20260600002');
INSERT IGNORE INTO askxuan_product.product_intent_tag(product_id,tag_code)
SELECT id,'peace' FROM askxuan_product.product WHERE product_no='P20260600001';
INSERT IGNORE INTO askxuan_temple.temple_service_intent_tag(temple_service_id,tag_code)
SELECT id, CASE service_code
  WHEN 'S001' THEN 'peace' WHEN 'S002' THEN 'love' WHEN 'S003' THEN 'wealth'
  WHEN 'S005' THEN 'rite' WHEN 'S006' THEN 'career' WHEN 'S007' THEN 'taisui'
  WHEN 'S008' THEN 'love' WHEN 'S009' THEN 'wealth' WHEN 'S010' THEN 'career'
  WHEN 'S011' THEN 'career' WHEN 'S012' THEN 'peace' WHEN 'S013' THEN 'study'
END FROM askxuan_temple.temple_service WHERE service_code IN ('S001','S002','S003','S005','S006','S007','S008','S009','S010','S011','S012','S013');

GRANT SELECT ON askxuan_temple.temple TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service TO 'product_user'@'%';
GRANT SELECT ON askxuan_temple.temple_service_intent_tag TO 'product_user'@'%';
FLUSH PRIVILEGES;
