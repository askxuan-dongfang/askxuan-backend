-- Both schemas live on the same MySQL instance: debit, ledger, code and prize
-- reservation commit together. Do not grant access to cash payments or growth.
GRANT SELECT, UPDATE (balance) ON askxuan_payment.points_account TO 'marketing_user'@'%';
GRANT SELECT, INSERT ON askxuan_payment.points_ledger TO 'marketing_user'@'%';
