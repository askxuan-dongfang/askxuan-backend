-- Apply before deploying auth/user services. Existing accounts are never linked by unverified contact data.
CREATE TABLE IF NOT EXISTS askxuan_auth.auth_identity (
 domain VARCHAR(16) NOT NULL,
 user_id BIGINT NOT NULL,
 username VARCHAR(32) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
 email VARCHAR(254) CHARACTER SET ascii COLLATE ascii_general_ci NOT NULL,
 password_hash VARCHAR(255) NOT NULL,
 agreement_version VARCHAR(32) NOT NULL DEFAULT '',
 verified_at DATETIME NOT NULL,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
 PRIMARY KEY(domain,user_id),
 UNIQUE KEY uk_identity_username(domain,username),
 UNIQUE KEY uk_identity_email(domain,email)
) ENGINE=InnoDB;
ALTER TABLE askxuan_user.user MODIFY mobile VARCHAR(20) NULL;
ALTER TABLE askxuan_user.user MODIFY password VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE askxuan_auth.admin_account MODIFY password VARCHAR(255) NOT NULL;
