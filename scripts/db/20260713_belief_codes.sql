-- Requirement 5: canonical belief categories. Safe to run repeatedly.
-- Existing installations may still keep the core tables in askxuan, so first
-- seed the service databases before adding the new columns.
CREATE DATABASE IF NOT EXISTS askxuan_temple CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS askxuan_master CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE TABLE IF NOT EXISTS askxuan_temple.temple LIKE askxuan.temple;
INSERT IGNORE INTO askxuan_temple.temple (id,code,name,region,type,sect,status,address,cover_image,rating,description,create_time,update_time)
SELECT id,code,name,region,type,sect,status,address,cover_image,rating,description,create_time,update_time FROM askxuan.temple;
CREATE TABLE IF NOT EXISTS askxuan_master.master LIKE askxuan.master;
INSERT IGNORE INTO askxuan_master.master (id,code,dharma_name,lay_name,temple_code,position,sect,type,auth_status,shelf_status,platform_status,specialties,avatar,rating,create_time,update_time)
SELECT id,code,dharma_name,lay_name,temple_code,position,sect,type,auth_status,shelf_status,platform_status,specialties,avatar,rating,create_time,update_time FROM askxuan.master;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='temple' AND column_name='belief_code'),
  'SELECT 1',
  "ALTER TABLE askxuan_temple.temple ADD COLUMN belief_code VARCHAR(32) NOT NULL DEFAULT 'han_buddhism' AFTER type"
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema='askxuan_temple' AND table_name='temple' AND index_name='idx_belief_code'),
  'SELECT 1',
  'CREATE INDEX idx_belief_code ON askxuan_temple.temple(belief_code)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='askxuan_master' AND table_name='master' AND column_name='belief_code'),
  'SELECT 1',
  "ALTER TABLE askxuan_master.master ADD COLUMN belief_code VARCHAR(32) NOT NULL DEFAULT 'han_buddhism' AFTER position"
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema='askxuan_master' AND table_name='master' AND index_name='idx_belief_code'),
  'SELECT 1',
  'CREATE INDEX idx_belief_code ON askxuan_master.master(belief_code)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE askxuan_temple.temple SET belief_code = CASE
  WHEN type LIKE '%藏传%' OR sect IN ('格鲁派','宁玛派','噶举派','萨迦派') THEN 'tibetan_buddhism'
  WHEN type LIKE '%道教%' OR sect IN ('全真派','正一派') THEN 'daoism'
  WHEN type LIKE '%民间%' THEN 'folk'
  ELSE 'han_buddhism' END;
UPDATE askxuan_master.master SET belief_code = CASE
  WHEN type LIKE '%藏%' OR sect IN ('格鲁派','宁玛派','噶举派','萨迦派') THEN 'tibetan_buddhism'
  WHEN type LIKE '%道教%' OR sect IN ('全真派','全真道派','正一派') THEN 'daoism'
  WHEN type LIKE '%民间%' THEN 'folk'
  ELSE 'han_buddhism' END;

CREATE TABLE IF NOT EXISTS askxuan_temple.belief_profile (
  code VARCHAR(32) NOT NULL PRIMARY KEY,
  name VARCHAR(64) NOT NULL,
  summary VARCHAR(255) NOT NULL DEFAULT '',
  description TEXT NOT NULL,
  cover_image VARCHAR(500) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'enabled',
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
INSERT INTO askxuan_temple.belief_profile(code,name,summary,description,sort) VALUES
('han_buddhism','汉传佛教','慈悲与智慧并行','汉传佛教在中国长期发展，形成禅、净土、天台、华严等具体宗派。',10),
('tibetan_buddhism','藏传佛教','传承、修持与慈悲','藏传佛教具有清晰的传承体系，包含格鲁、宁玛、噶举、萨迦等具体宗派。',20),
('daoism','道教','道法自然，清静修持','道教是中国本土宗教传统，包含全真、正一等具体宗派。',30),
('folk','民间信仰','乡土传统与民俗传承','民间信仰承载地域性祭祀、祈愿和文化传统。',40)
ON DUPLICATE KEY UPDATE name=VALUES(name), summary=VALUES(summary), description=VALUES(description), sort=VALUES(sort);
