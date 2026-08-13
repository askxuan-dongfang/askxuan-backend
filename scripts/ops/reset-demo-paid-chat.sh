#!/usr/bin/env bash
# Reset one demo customer's paid-booking chat chain after taking a database backup.

set -euo pipefail

MYSQL_CONTAINER="${MYSQL_CONTAINER:-askxuan-mysql}"
USER_ID="${USER_ID:-1}"
TEMPLE_CODE="${TEMPLE_CODE:-T001}"
MASTER_CODE="${MASTER_CODE:-M001}"
BACKUP_DIR="${BACKUP_DIR:-/opt/askxuan/backups}"
SECRETS_FILE="${SECRETS_FILE:-/opt/askxuan/runtime/secrets.env}"

mysql_password="${MYSQL_ROOT_PASSWORD:-}"
if [ -z "$mysql_password" ] && [ -r "$SECRETS_FILE" ]; then
  mysql_password="$(sed -n 's/^MYSQL_ROOT_PASSWORD=//p' "$SECRETS_FILE" | tail -1)"
fi
if [ -z "$mysql_password" ]; then
  mysql_password="$(docker inspect "$MYSQL_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' \
    | sed -n 's/^MYSQL_ROOT_PASSWORD=//p' | head -1)"
fi
if [ -z "$mysql_password" ]; then
  echo "MYSQL_ROOT_PASSWORD is not available from $MYSQL_CONTAINER" >&2
  exit 1
fi

timestamp="$(date +%Y%m%d-%H%M%S)"
backup_path="$BACKUP_DIR/paid-chat-${USER_ID}-${TEMPLE_CODE}-${MASTER_CODE}-${timestamp}.sql.gz"
mkdir -p "$BACKUP_DIR"

docker exec -e MYSQL_PWD="$mysql_password" "$MYSQL_CONTAINER" mysqldump -uroot \
  --single-transaction --quick --databases \
  askxuan askxuan_shop askxuan_booking askxuan_payment askxuan_finance \
  askxuan_master askxuan_message askxuan_review \
  | gzip >"$backup_path"

docker exec -e MYSQL_PWD="$mysql_password" -i "$MYSQL_CONTAINER" \
  mysql -uroot --default-character-set=utf8mb4 \
  --init-command="SET @target_user='${USER_ID}', @target_temple='${TEMPLE_CODE}', @target_master='${MASTER_CODE}'" <<'SQL'
SET NAMES utf8mb4;
USE askxuan_booking;
START TRANSACTION;

CREATE TEMPORARY TABLE target_booking AS
SELECT booking_no,temple_code,service_code,booking_date,slot_code
FROM askxuan_booking.booking
WHERE user_id=@target_user AND temple_code=@target_temple AND master_code=@target_master;

CREATE TEMPORARY TABLE target_slot AS
SELECT DISTINCT temple_code,service_code,booking_date,slot_code
FROM target_booking WHERE slot_code<>'';

CREATE TEMPORARY TABLE target_review AS
SELECT id FROM askxuan_review.review
WHERE user_id=@target_user AND target_type='booking'
  AND target_id IN (SELECT booking_no FROM target_booking);

CREATE TEMPORARY TABLE target_settlement AS
SELECT id FROM askxuan_finance.settlement
WHERE source_type='booking' AND source_no IN (SELECT booking_no FROM target_booking);

CREATE TEMPORARY TABLE target_transaction AS
SELECT id FROM askxuan_finance.finance_transaction
WHERE source_type='booking' AND source_no IN (SELECT booking_no FROM target_booking);

CREATE TEMPORARY TABLE target_payment AS
SELECT id FROM askxuan_payment.payment
WHERE order_type='booking' AND order_no IN (SELECT booking_no FROM target_booking);

DELETE rr FROM askxuan_review.review_reply rr JOIN target_review r ON r.id=rr.review_id;
DELETE rp FROM askxuan_review.review_report rp JOIN target_review r ON r.id=rp.review_id;
DELETE r FROM askxuan_review.review r JOIN target_review tr ON tr.id=r.id;

DELETE FROM askxuan_message.message
WHERE user_id=@target_user AND biz_type='booking'
  AND biz_id IN (SELECT booking_no FROM target_booking);
DELETE FROM askxuan_message.push_log
WHERE user_id=@target_user AND biz_type='booking'
  AND biz_id IN (SELECT booking_no FROM target_booking);

DELETE FROM askxuan_master.master_earning
WHERE source_type='booking' AND source_id IN (SELECT booking_no FROM target_booking);

DELETE fl FROM askxuan_finance.finance_log fl JOIN target_settlement s ON s.id=fl.settlement_id;
DELETE s FROM askxuan_finance.settlement s JOIN target_settlement ts ON ts.id=s.id;
DELETE le FROM askxuan_finance.finance_ledger_entry le JOIN target_transaction t ON t.id=le.transaction_id;
DELETE ft FROM askxuan_finance.finance_transaction ft JOIN target_transaction t ON t.id=ft.id;

DELETE rf FROM askxuan_payment.refund rf JOIN target_payment p ON p.id=rf.payment_id;
DELETE pl FROM askxuan_payment.payment_log pl JOIN target_payment p ON p.id=pl.payment_id;
DELETE p FROM askxuan_payment.payment p JOIN target_payment tp ON tp.id=p.id;

DELETE c FROM askxuan_booking.booking_chat_message c JOIN target_booking b ON b.booking_no=c.booking_id;
DELETE r FROM askxuan_booking.booking_review r JOIN target_booking b ON b.booking_no=r.booking_id;
DELETE l FROM askxuan_booking.booking_status_log l JOIN target_booking b ON b.booking_no=l.booking_id;
DELETE b FROM askxuan_booking.booking b JOIN target_booking t ON t.booking_no=b.booking_no;

UPDATE askxuan_booking.booking_slot_inventory i
JOIN target_slot t ON t.temple_code=i.temple_code AND t.service_code=i.service_code
  AND t.booking_date=i.booking_date AND t.slot_code=i.slot_code
SET i.reserved_count=(
  SELECT COUNT(*) FROM askxuan_booking.booking b
  WHERE b.temple_code=i.temple_code AND b.service_code=i.service_code
    AND b.booking_date=i.booking_date AND b.slot_code=i.slot_code AND b.slot_reserved=1
);
DELETE i FROM askxuan_booking.booking_slot_inventory i
JOIN target_slot t ON t.temple_code=i.temple_code AND t.service_code=i.service_code
  AND t.booking_date=i.booking_date AND t.slot_code=i.slot_code
WHERE i.reserved_count=0;

-- Remove compatibility copies so a future migration cannot resurrect stale demo data.
DELETE pl FROM askxuan_shop.payment_log pl
JOIN askxuan_shop.payment p ON p.id=pl.payment_id
WHERE p.order_type='booking' AND p.order_no IN (SELECT booking_no FROM target_booking);
DELETE rf FROM askxuan_shop.refund rf
JOIN askxuan_shop.payment p ON p.id=rf.payment_id
WHERE p.order_type='booking' AND p.order_no IN (SELECT booking_no FROM target_booking);
DELETE FROM askxuan_shop.payment
WHERE order_type='booking' AND order_no IN (SELECT booking_no FROM target_booking);
DELETE FROM askxuan.message
WHERE user_id=@target_user AND biz_type='booking'
  AND biz_id IN (SELECT booking_no FROM target_booking);
DELETE FROM askxuan.booking WHERE booking_no IN (SELECT booking_no FROM target_booking);

COMMIT;

SELECT 'remaining_bookings',COUNT(*) FROM askxuan_booking.booking
WHERE user_id=@target_user AND temple_code=@target_temple AND master_code=@target_master;
SELECT 'remaining_chat_messages',COUNT(*) FROM askxuan_booking.booking_chat_message c
JOIN askxuan_booking.booking b ON b.booking_no=c.booking_id
WHERE b.user_id=@target_user AND b.temple_code=@target_temple AND b.master_code=@target_master;
SQL

echo "Backup: $backup_path"
