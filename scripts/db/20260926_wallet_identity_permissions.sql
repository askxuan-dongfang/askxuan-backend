-- Resolve authenticated numeric master IDs to settlement business codes.
-- Read only the two identity columns; no profile or write access is granted.
GRANT SELECT (id, code) ON askxuan_master.master TO 'finance_user'@'%';
