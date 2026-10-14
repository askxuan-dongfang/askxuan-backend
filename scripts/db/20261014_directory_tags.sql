-- Additive shared dictionary. Existing city, sect, position and specialties remain intact.
CREATE TABLE IF NOT EXISTS askxuan_temple.directory_tag (
 code VARCHAR(32) NOT NULL PRIMARY KEY,
 tag_group VARCHAR(16) NOT NULL,
 name VARCHAR(32) NOT NULL,
 value VARCHAR(64) NOT NULL,
 sort INT NOT NULL DEFAULT 0,
 status VARCHAR(16) NOT NULL DEFAULT 'enabled',
 UNIQUE KEY uk_directory_value(tag_group,value)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
CREATE TABLE IF NOT EXISTS askxuan_temple.provider_directory_tag (
 provider_kind VARCHAR(16) NOT NULL,
 provider_code VARCHAR(64) NOT NULL,
 tag_code VARCHAR(32) NOT NULL,
 PRIMARY KEY(provider_kind,provider_code,tag_code),
 KEY idx_directory_provider(tag_code,provider_kind,provider_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
GRANT SELECT ON askxuan_temple.directory_tag TO 'master_user'@'%';
GRANT SELECT ON askxuan_temple.provider_directory_tag TO 'master_user'@'%';
