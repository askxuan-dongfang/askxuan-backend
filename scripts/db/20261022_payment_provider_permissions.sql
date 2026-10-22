-- Read current provider eligibility before charging existing service orders.
-- SELECT FOR SHARE serializes real charges with subject platform-state changes.
GRANT SELECT(code,auth_status,platform_status,temple_code) ON askxuan_master.master TO 'payment_user'@'%';
GRANT SELECT(code,auth_status,platform_status) ON askxuan_temple.temple TO 'payment_user'@'%';
