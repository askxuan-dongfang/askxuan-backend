-- Run as the database administrator after the report schema and service users exist.
-- AI only reads payment entitlement; payment can only change the report paid flag.
GRANT SELECT ON askxuan_payment.payment TO 'ai_user'@'%';
GRANT SELECT, UPDATE (points_paid) ON askxuan_ai.ai_report TO 'payment_user'@'%';
