-- Additive migration: keep all existing accounts and business identifiers.
INSERT INTO askxuan_auth.role(name,code,description) VALUES
('大师认证申请人','master_applicant','仅可管理本人独立大师认证申请'),
('寺院入驻申请人','temple_applicant','仅可管理本人寺院入驻申请')
ON DUPLICATE KEY UPDATE description=VALUES(description);
CREATE TABLE IF NOT EXISTS askxuan_auth.onboarding_application (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 account_id BIGINT NOT NULL UNIQUE,
 kind VARCHAR(16) NOT NULL,
 status VARCHAR(16) NOT NULL DEFAULT 'draft',
 profile_json TEXT NOT NULL,
 revision INT NOT NULL DEFAULT 1,
 review_note VARCHAR(1000) NOT NULL DEFAULT '',
 entity_code VARCHAR(16) NOT NULL DEFAULT '',
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 KEY idx_queue(status,id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_auth.onboarding_event (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 application_id BIGINT NOT NULL,
 actor_id BIGINT NOT NULL,
 action VARCHAR(32) NOT NULL,
 revision INT NOT NULL,
 note VARCHAR(1000) NOT NULL DEFAULT '',
 profile_json TEXT NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY idx_application(application_id,id)
) ENGINE=InnoDB;
-- Private evidence is never exposed through public object storage or listings.
CREATE TABLE IF NOT EXISTS askxuan_auth.onboarding_evidence (
 id VARCHAR(48) PRIMARY KEY,
 account_id BIGINT NOT NULL,
 filename VARCHAR(180) NOT NULL,
 media_type VARCHAR(64) NOT NULL,
 content MEDIUMBLOB NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 KEY idx_owner(account_id)
) ENGINE=InnoDB;
CREATE TABLE IF NOT EXISTS askxuan_auth.master_invitation (
 id BIGINT PRIMARY KEY AUTO_INCREMENT,
 account_id BIGINT NOT NULL UNIQUE,
 temple_code VARCHAR(16) NOT NULL,
 master_code VARCHAR(16) NOT NULL UNIQUE,
 email VARCHAR(254) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL UNIQUE,
 username VARCHAR(32) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL UNIQUE,
 status VARCHAR(16) NOT NULL DEFAULT 'pending',
 created_by BIGINT NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB;
