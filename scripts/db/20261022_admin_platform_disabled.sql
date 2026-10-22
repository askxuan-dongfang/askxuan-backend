-- Apply after admin_rbac and BEFORE new auth deployment. Additive; no credentials/roles/bindings changed.
-- Existing disabled accounts have unknown historical ownership: conservatively
-- preserve disabled and require explicit platform release before activation/restore.
USE askxuan_auth;
SET @add_platform_disabled=(SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='admin_account' AND column_name='platform_disabled')=0;
SET @ddl=IF(@add_platform_disabled, 'ALTER TABLE admin_account ADD COLUMN platform_disabled TINYINT(1) NOT NULL DEFAULT 0', 'SELECT 1');
PREPARE stmt FROM @ddl; EXECUTE stmt; DEALLOCATE PREPARE stmt;
UPDATE admin_account SET platform_disabled=1,authz_version=authz_version+1 WHERE @add_platform_disabled AND status='disabled';

-- Repair an inconsistent pre-existing flag without changing any enabled account
-- that has not been explicitly blocked. Re-running is a no-op.
UPDATE admin_account SET status='disabled',authz_version=authz_version+1 WHERE platform_disabled=1 AND status<>'disabled';
