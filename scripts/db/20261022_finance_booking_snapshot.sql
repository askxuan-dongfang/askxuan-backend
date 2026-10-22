-- Read only the immutable snapshot needed to consume older reviewed events.
GRANT SELECT (booking_no,user_id,temple_code,temple_name,master_code,master_name,service_name,booking_date,service_fee,merit_money,total_fee,status,payment_status)
ON askxuan_booking.booking TO 'finance_user'@'%';
