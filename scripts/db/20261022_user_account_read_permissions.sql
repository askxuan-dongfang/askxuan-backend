-- Read only projections for super-administrator consumer account details.
-- No password/hash access and no writes on payment or identity data.
GRANT SELECT(id,user_id,order_type,order_no,amount,channel,status) ON askxuan_payment.payment TO 'user_user'@'%';
GRANT SELECT(payment_id,amount,status) ON askxuan_payment.refund TO 'user_user'@'%';
GRANT SELECT(domain,user_id,email,verified_at) ON askxuan_auth.auth_identity TO 'user_user'@'%';
