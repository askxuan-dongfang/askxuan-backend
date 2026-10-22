-- Additive, one-time reviewed migration. No accounts, business IDs or assets are replaced.
-- Legacy banned rows have no reliable certification history: keep them explicitly unknown.
ALTER TABLE askxuan_temple.temple
 ADD COLUMN auth_status VARCHAR(24) NOT NULL DEFAULT 'pending',
 ADD COLUMN platform_status VARCHAR(16) NOT NULL DEFAULT 'normal',
 ADD COLUMN shelf_status VARCHAR(16) NOT NULL DEFAULT 'on_shelf',
 ADD COLUMN recommended TINYINT(1) NOT NULL DEFAULT 0,
 ADD COLUMN state_version BIGINT NOT NULL DEFAULT 0;
UPDATE askxuan_temple.temple SET
 auth_status=CASE WHEN status IN ('正常','推荐') THEN 'certified' WHEN status='封禁' THEN 'legacy_unknown' ELSE 'pending' END,
 platform_status=IF(status='封禁','banned','normal'),
 shelf_status=IF(status IN ('正常','推荐'),'on_shelf','off_shelf'),
 recommended=IF(status='推荐',1,0);
-- Apply the minimal new privilege separately after inspecting SHOW GRANTS.
-- GRANT SELECT(id,temple_id,authz_version),UPDATE(authz_version) ON askxuan_auth.admin_account TO 'temple_user'@'%';
-- Other service readers must explicitly opt into new-display gating.
-- GRANT SELECT(auth_status,platform_status,shelf_status) ON askxuan_temple.temple TO 'diy_user'@'%';
-- GRANT SELECT(id,master_id,authz_version),UPDATE(authz_version) ON askxuan_auth.admin_account TO 'master_user'@'%';
-- GRANT SELECT(id,status) ON askxuan_user.user TO 'booking_user'@'%';
