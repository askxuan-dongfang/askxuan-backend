-- Additive minimum grants after SHOW GRANTS and deployed DSN user/host checks.
-- No credentials are changed; state and affected work-session versions share a TX.
GRANT SELECT(id,temple_id,authz_version),UPDATE(authz_version) ON askxuan_auth.admin_account TO 'temple_user'@'%';
GRANT SELECT(id,master_id,authz_version),UPDATE(authz_version) ON askxuan_auth.admin_account TO 'master_user'@'%';
GRANT SELECT(id,status) ON askxuan_user.user TO 'booking_user'@'%';
GRANT SELECT(auth_status,platform_status,shelf_status) ON askxuan_temple.temple TO 'auth_user'@'%';
GRANT SELECT(code,auth_status,platform_status) ON askxuan_temple.temple TO 'master_user'@'%';
GRANT SELECT(auth_status,platform_status,shelf_status) ON askxuan_temple.temple TO 'diy_user'@'%';
