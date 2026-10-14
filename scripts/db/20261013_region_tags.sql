-- Additive migration; retain the existing city/address text and all existing assignments.
CREATE TABLE IF NOT EXISTS askxuan_temple.region_tag (
 code VARCHAR(32) NOT NULL PRIMARY KEY,
 name VARCHAR(32) NOT NULL,
 sort INT NOT NULL DEFAULT 0,
 status VARCHAR(16) NOT NULL DEFAULT 'enabled',
 UNIQUE KEY uk_region_name(name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE IF NOT EXISTS askxuan_temple.provider_region (
 provider_kind VARCHAR(16) NOT NULL,
 provider_code VARCHAR(64) NOT NULL,
 region_code VARCHAR(32) NOT NULL,
 PRIMARY KEY(provider_kind,provider_code),
 KEY idx_region_provider(region_code,provider_kind,provider_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
INSERT IGNORE INTO askxuan_temple.region_tag(code,name,sort,status) VALUES
 ('mainland','大陆',10,'enabled'),('hong_kong','香港',20,'enabled'),
 ('macau','澳门',30,'enabled'),('taiwan','台湾',40,'enabled'),
 ('southeast_asia','东南亚',50,'enabled');
-- Backfill only explicit city/province prefixes. Never guess an independent master's location.
INSERT IGNORE INTO askxuan_temple.provider_region(provider_kind,provider_code,region_code)
 SELECT 'temple',code,CASE
 WHEN region REGEXP '^(中国)?香港' THEN 'hong_kong'
 WHEN region REGEXP '^(中国)?澳门' THEN 'macau'
 WHEN region REGEXP '^(中国)?台湾' THEN 'taiwan'
 WHEN region REGEXP '^(新加坡|马来西亚|泰国|越南|印度尼西亚|印尼|菲律宾|缅甸|柬埔寨|老挝|文莱|东帝汶)' THEN 'southeast_asia'
 ELSE 'mainland' END
 FROM askxuan_temple.temple
 WHERE region REGEXP '^(中国)?(香港|澳门|台湾|北京|天津|上海|重庆|河北|山西|辽宁|吉林|黑龙江|江苏|浙江|安徽|福建|江西|山东|河南|湖北|湖南|广东|海南|四川|贵州|云南|陕西|甘肃|青海|内蒙古|广西|西藏|宁夏|新疆)' OR region REGEXP '^(新加坡|马来西亚|泰国|越南|印度尼西亚|印尼|菲律宾|缅甸|柬埔寨|老挝|文莱|东帝汶)';

-- Only the cross-service reads needed for region inheritance and provider validation.
GRANT SELECT ON askxuan_temple.region_tag TO 'master_user'@'%';
GRANT SELECT ON askxuan_temple.provider_region TO 'master_user'@'%';
GRANT SELECT (code) ON askxuan_master.master TO 'temple_user'@'%';
