-- Additive minimum table permissions for the existing service identities.
-- Run only after verifying these user/host pairs against the deployed DSNs.
-- No credentials, accounts, catalog content or provider data are changed.
-- Repeated GRANT is safe. Preserve prior SHOW GRANTS before applying.
GRANT SELECT ON askxuan_temple.directory_tag TO 'auth_user'@'%';
GRANT SELECT, INSERT ON askxuan_temple.provider_directory_tag TO 'auth_user'@'%';
GRANT SELECT ON askxuan_temple.directory_tag TO 'master_user'@'%';
GRANT SELECT, INSERT, DELETE ON askxuan_temple.provider_directory_tag TO 'master_user'@'%';
-- Existing region/provider validation already grants SELECT(code). Standard
-- affiliation validation additionally needs only the primary classification.
GRANT SELECT (belief_code) ON askxuan_master.master TO 'temple_user'@'%';
