-- Default rules match the existing live service/wish mappings. Preserve custom bindings.
SET NAMES utf8mb4;
CREATE TABLE IF NOT EXISTS askxuan_product.intention_migration (version VARCHAR(64) PRIMARY KEY);
CREATE TABLE IF NOT EXISTS askxuan_product.master_service_intention (
 master_code VARCHAR(32) NOT NULL, service_code VARCHAR(32) NOT NULL, tag_code VARCHAR(32) NOT NULL,
 PRIMARY KEY(master_code,service_code,tag_code)
);
CREATE TABLE IF NOT EXISTS askxuan_product.service_intention_rule (
 service_code VARCHAR(32) NOT NULL, tag_code VARCHAR(32) NOT NULL,
 PRIMARY KEY(service_code,tag_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
SET @q=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='temple_service' AND column_name='intent_mode')=0,
 'ALTER TABLE askxuan_temple.temple_service ADD COLUMN intent_mode VARCHAR(16) NOT NULL DEFAULT ''inherit''','SELECT 1');
PREPARE s FROM @q; EXECUTE s; DEALLOCATE PREPARE s;
SET @q=IF((SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_master' AND table_name='master_service_tag' AND column_name='intent_mode')=0,
 'ALTER TABLE askxuan_master.master_service_tag ADD COLUMN intent_mode VARCHAR(16) NOT NULL DEFAULT ''inherit''','SELECT 1');
PREPARE s FROM @q; EXECUTE s; DEALLOCATE PREPARE s;
START TRANSACTION;
INSERT IGNORE INTO askxuan_product.service_intention_rule(service_code,tag_code)
SELECT service_code,tag_code FROM (
 SELECT 'S001' service_code,'peace' tag_code UNION ALL
 SELECT 'S002','peace' UNION ALL SELECT 'S002','love' UNION ALL
 SELECT 'S003','peace' UNION ALL SELECT 'S003','wealth' UNION ALL
 SELECT 'S004','rite' UNION ALL SELECT 'S005','rite' UNION ALL
 SELECT 'S006','career' UNION ALL SELECT 'S007','taisui' UNION ALL
 SELECT 'S008','love' UNION ALL SELECT 'S009','wealth' UNION ALL
 SELECT 'S010','career' UNION ALL SELECT 'S011','career' UNION ALL
 SELECT 'S012','peace' UNION ALL SELECT 'S013','study'
) defaults_seed
WHERE NOT EXISTS(SELECT 1 FROM askxuan_product.intention_migration WHERE version='20261014-defaults');
UPDATE askxuan_temple.temple_service s SET s.intent_mode='custom'
WHERE EXISTS(SELECT 1 FROM askxuan_temple.temple_service_intent_tag t WHERE t.temple_service_id=s.id)
AND NOT EXISTS(SELECT 1 FROM askxuan_product.intention_migration WHERE version='20261014-defaults');
UPDATE askxuan_master.master_service_tag s SET s.intent_mode='custom'
WHERE EXISTS(SELECT 1 FROM askxuan_product.master_service_intention t WHERE t.master_code=s.master_code AND t.service_code=s.service_code)
AND NOT EXISTS(SELECT 1 FROM askxuan_product.intention_migration WHERE version='20261014-defaults');
INSERT IGNORE INTO askxuan_product.intention_migration VALUES('20261014-defaults');
COMMIT;
GRANT UPDATE(intent_mode) ON askxuan_temple.temple_service TO 'product_user'@'%';
GRANT UPDATE(intent_mode) ON askxuan_master.master_service_tag TO 'product_user'@'%';
GRANT SELECT ON askxuan_product.service_intention_rule TO 'temple_user'@'%';
GRANT SELECT ON askxuan_product.intent_tag TO 'temple_user'@'%';
