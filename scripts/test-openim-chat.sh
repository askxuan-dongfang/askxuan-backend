#!/usr/bin/env bash
# Paid-booking chat acceptance: entitlement, both roles, persistence and OpenIM callbacks.

set -euo pipefail

GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
OPENIM="${OPENIM:-http://127.0.0.1:10002}"
BOOKING_DIRECT="${BOOKING_DIRECT:-http://127.0.0.1:8085}"
OPENIM_SECRET="${OPENIM_SECRET:-openIM123}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-root123}"
PASS=0
FAIL=0
REQUEST_ID="chat-accept-$(date +%s)-$RANDOM"
BOOKING_ID=""

pass() { PASS=$((PASS + 1)); printf '[PASS] %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf '[FAIL] %s\n' "$1"; }

future_date() {
  local days="$1"
  if date -v+"${days}"d +%F >/dev/null 2>&1; then
    date -v+"${days}"d +%F
  else
    date -d "+${days} days" +%F
  fi
}

cleanup() {
  docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" -i askxuan-mysql mysql -uroot 2>/dev/null <<SQL || true
CREATE TEMPORARY TABLE cleanup_chat_booking AS
SELECT booking_no,temple_code,service_code,booking_date,slot_code
FROM askxuan_booking.booking WHERE request_id='${REQUEST_ID}';
DELETE c FROM askxuan_booking.booking_chat_message c JOIN cleanup_chat_booking b ON b.booking_no=c.booking_id;
DELETE l FROM askxuan_booking.booking_status_log l JOIN cleanup_chat_booking b ON b.booking_no=l.booking_id;
DELETE e FROM askxuan_master.master_earning e JOIN cleanup_chat_booking b ON b.booking_no=e.source_id WHERE e.source_type='booking';
DELETE r FROM askxuan_review.review r JOIN cleanup_chat_booking b ON b.booking_no=r.target_id WHERE r.target_type='booking';
DELETE r FROM askxuan_booking.booking_review r JOIN cleanup_chat_booking b ON b.booking_no=r.booking_id;
DELETE le FROM askxuan_finance.finance_ledger_entry le
JOIN askxuan_finance.finance_transaction ft ON ft.id=le.transaction_id
JOIN cleanup_chat_booking b ON b.booking_no=ft.source_no WHERE ft.source_type='booking';
DELETE s FROM askxuan_finance.settlement s JOIN cleanup_chat_booking b ON b.booking_no=s.source_no WHERE s.source_type='booking';
DELETE fl FROM askxuan_finance.finance_log fl JOIN cleanup_chat_booking b ON fl.description LIKE CONCAT('%',b.booking_no,'%');
DELETE ft FROM askxuan_finance.finance_transaction ft JOIN cleanup_chat_booking b ON b.booking_no=ft.source_no WHERE ft.source_type='booking';
DELETE pl FROM askxuan_payment.payment_log pl
JOIN askxuan_payment.payment p ON p.id=pl.payment_id
JOIN cleanup_chat_booking b ON b.booking_no=p.order_no WHERE p.order_type='booking';
DELETE p FROM askxuan_payment.payment p
JOIN cleanup_chat_booking b ON b.booking_no=p.order_no WHERE p.order_type='booking';
DELETE b FROM askxuan_booking.booking b JOIN cleanup_chat_booking c ON c.booking_no=b.booking_no;
UPDATE askxuan_booking.booking_slot_inventory i
JOIN cleanup_chat_booking c ON c.temple_code=i.temple_code AND c.service_code=i.service_code
  AND c.booking_date=i.booking_date AND c.slot_code=i.slot_code
SET i.reserved_count=(
  SELECT COUNT(*) FROM askxuan_booking.booking b
  WHERE b.temple_code=i.temple_code AND b.service_code=i.service_code
    AND b.booking_date=i.booking_date AND b.slot_code=i.slot_code AND b.slot_reserved=1
);
DELETE i FROM askxuan_booking.booking_slot_inventory i
JOIN cleanup_chat_booking c ON c.temple_code=i.temple_code AND c.service_code=i.service_code
  AND c.booking_date=i.booking_date AND c.slot_code=i.slot_code
WHERE i.reserved_count=0;
DROP TEMPORARY TABLE cleanup_chat_booking;
SQL
}
trap cleanup EXIT

command -v jq >/dev/null 2>&1 || { printf 'jq is required\n' >&2; exit 1; }

printf '===== Paid booking chat acceptance =====\n'

ADMIN_RESP=$(curl -fsS --max-time 10 -X POST "$OPENIM/auth/get_admin_token" \
  -H 'Content-Type: application/json' -H 'operationID: chat-admin-token' \
  -d "{\"secret\":\"$OPENIM_SECRET\",\"userID\":\"imAdmin\"}")
ADMIN_TOKEN=$(printf '%s' "$ADMIN_RESP" | jq -r '.data.token // .token // empty')
if [ -n "$ADMIN_TOKEN" ]; then pass 'OpenIM admin API is reachable'; else fail 'OpenIM admin token is empty'; fi

CUSTOMER_LOGIN=$(curl -fsS --max-time 10 -X POST "$GATEWAY/api/v1/auth/login" \
  -H 'Content-Type: application/json' -d '{"phone":"13800138000","password":"123456"}')
CUSTOMER_TOKEN=$(printf '%s' "$CUSTOMER_LOGIN" | jq -r '.data.accessToken // empty')
CUSTOMER_IM_TOKEN=$(printf '%s' "$CUSTOMER_LOGIN" | jq -r '.data.imToken // empty')
USER_ID=$(printf '%s' "$CUSTOMER_LOGIN" | jq -r '.data.userInfo.userId // empty')
if [ -n "$CUSTOMER_TOKEN" ] && [ -n "$CUSTOMER_IM_TOKEN" ] && [ -n "$USER_ID" ]; then
  pass 'customer login returns API and OpenIM tokens'
else
  fail 'customer login contract mismatch'
fi

MASTER_LOGIN=$(curl -fsS --max-time 10 -X POST "$GATEWAY/api/v1/auth/admin/login" \
  -H 'Content-Type: application/json' -d '{"account":"zhihai","password":"123456"}')
MASTER_TOKEN=$(printf '%s' "$MASTER_LOGIN" | jq -r '.data.accessToken // empty')
MASTER_IM_TOKEN=$(printf '%s' "$MASTER_LOGIN" | jq -r '.data.imToken // empty')
if [ -n "$MASTER_TOKEN" ] && [ -n "$MASTER_IM_TOKEN" ]; then
  pass 'master login returns API and OpenIM tokens'
else
  fail 'master login contract mismatch'
fi

BOOKING_DATE=$(future_date "$((720 + RANDOM % 180))")
AVAIL=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/bookings/availability?templeId=T001&serviceId=S001&date=$BOOKING_DATE")
SLOT_CODE=$(printf '%s' "$AVAIL" | jq -r '.data.slots[] | select(.available == true) | .slotCode' | head -1)
CREATE_PAYLOAD=$(jq -nc --arg requestId "$REQUEST_ID" --arg date "$BOOKING_DATE" --arg slot "$SLOT_CODE" \
  '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:$slot,bookingDate:$date,meritMoney:1,meritMoneyTier:"custom"}')
CREATE=$(curl -fsS --max-time 20 -X POST "$GATEWAY/api/v1/bookings" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" -H 'Content-Type: application/json' -d "$CREATE_PAYLOAD")
BOOKING_ID=$(printf '%s' "$CREATE" | jq -r '.data.id // empty')
if [ -n "$BOOKING_ID" ] && [ "$(printf '%s' "$CREATE" | jq -r '.data.paymentStatus // empty')" = 'success' ]; then
  pass 'mock payment creates a paid booking entitlement'
else
  fail 'paid booking was not created'
fi

RECEIPT_POSTED=0
for _ in $(seq 1 15); do
  RECEIPT_POSTED=$(docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -uroot -Nse "
    SELECT IF(COUNT(DISTINCT ft.id)=1
      AND ROUND(SUM(IF(le.direction='debit',le.amount,0)),2)=MAX(b.total_fee)
      AND ROUND(SUM(IF(le.direction='credit',le.amount,0)),2)=MAX(b.total_fee),1,0)
    FROM askxuan_booking.booking b
    JOIN askxuan_finance.finance_transaction ft ON ft.source_type='booking' AND ft.source_no=b.booking_no AND ft.event_type='payment_receipt'
    JOIN askxuan_finance.finance_ledger_entry le ON le.transaction_id=ft.id
    WHERE b.booking_no='$BOOKING_ID';" 2>/dev/null || printf 0)
  [ "$RECEIPT_POSTED" = '1' ] && break
  sleep 1
done
if [ "$RECEIPT_POSTED" = '1' ]; then
  pass 'payment is posted as a balanced receipt in the platform general ledger'
else
  fail 'payment did not enter the platform general ledger'
fi

PRE_SETTLEMENT_EARNINGS=$(docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -uroot -Nse \
  "SELECT COUNT(*) FROM askxuan_master.master_earning WHERE source_type='booking' AND source_id='$BOOKING_ID';" 2>/dev/null || printf 1)
if [ "$PRE_SETTLEMENT_EARNINGS" = '0' ]; then
  pass 'payment does not directly credit the master account'
else
  fail 'master was credited before platform settlement'
fi

CUSTOMER_CHATS=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/bookings/chats?page=1&size=20" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN")
MASTER_CHATS=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/bookings/chats?page=1&size=20" \
  -H "Authorization: Bearer $MASTER_TOKEN")
if printf '%s' "$CUSTOMER_CHATS" | jq -e --arg id "$BOOKING_ID" '.data.list[] | select(.bookingId==$id and .canChat==true)' >/dev/null \
  && printf '%s' "$MASTER_CHATS" | jq -e --arg id "$BOOKING_ID" '.data.list[] | select(.bookingId==$id and .canChat==true)' >/dev/null; then
  pass 'the paid conversation is visible to both clients'
else
  fail 'conversation visibility differs between customer and master'
fi

CUSTOMER_MARKER="customer_chat_$(date +%s)_$RANDOM"
CUSTOMER_CLIENT_ID="ios-customer-$CUSTOMER_MARKER"
CUSTOMER_SEND=$(curl -fsS --max-time 15 -X POST "$GATEWAY/api/v1/bookings/$BOOKING_ID/chat/messages" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"clientMessageId\":\"$CUSTOMER_CLIENT_ID\",\"content\":\"$CUSTOMER_MARKER\"}")
CUSTOMER_MESSAGE_ID=$(printf '%s' "$CUSTOMER_SEND" | jq -r '.data.id // empty')
if [ -n "$CUSTOMER_MESSAGE_ID" ] && [ "$(printf '%s' "$CUSTOMER_SEND" | jq -r '.data.status // empty')" = 'sent' ]; then
  pass 'customer message is authorized and delivered through OpenIM'
else
  fail 'customer message delivery failed'
fi

DUPLICATE_SEND=$(curl -fsS --max-time 15 -X POST "$GATEWAY/api/v1/bookings/$BOOKING_ID/chat/messages" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"clientMessageId\":\"$CUSTOMER_CLIENT_ID\",\"content\":\"$CUSTOMER_MARKER\"}")
if [ "$(printf '%s' "$DUPLICATE_SEND" | jq -r '.data.id // empty')" = "$CUSTOMER_MESSAGE_ID" ]; then
  pass 'repeated clientMessageId is idempotent'
else
  fail 'message idempotency mismatch'
fi

MASTER_HISTORY=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/bookings/$BOOKING_ID/chat/messages?page=1&size=100" \
  -H "Authorization: Bearer $MASTER_TOKEN")
if printf '%s' "$MASTER_HISTORY" | jq -e --arg marker "$CUSTOMER_MARKER" '.data.list[] | select(.content==$marker and .senderType=="customer")' >/dev/null; then
  pass 'master reads the authoritative customer history'
else
  fail 'master cannot read the customer message'
fi

MASTER_MARKER="master_chat_$(date +%s)_$RANDOM"
MASTER_SEND=$(curl -fsS --max-time 15 -X POST "$GATEWAY/api/v1/bookings/$BOOKING_ID/chat/messages" \
  -H "Authorization: Bearer $MASTER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"clientMessageId\":\"ios-master-$MASTER_MARKER\",\"content\":\"$MASTER_MARKER\"}")
if [ "$(printf '%s' "$MASTER_SEND" | jq -r '.data.status // empty')" = 'sent' ]; then
  pass 'master reply is authorized and delivered through OpenIM'
else
  fail 'master reply delivery failed'
fi

CUSTOMER_HISTORY=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/bookings/$BOOKING_ID/chat/messages?page=1&size=100" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN")
if printf '%s' "$CUSTOMER_HISTORY" | jq -e --arg marker "$MASTER_MARKER" '.data.list[] | select(.content==$marker and .senderType=="master")' >/dev/null; then
  pass 'customer reads the authoritative master reply'
else
  fail 'customer cannot read the master reply'
fi

DENIED=$(curl -fsS --max-time 10 -X POST "$BOOKING_DIRECT/openim/booking-chat-webhook/beforeSendSingleMsg" \
  -H 'Content-Type: application/json' \
  -d "{\"callbackCommand\":\"beforeSendSingleMsg\",\"sendID\":\"u_$USER_ID\",\"recvID\":\"m_999999\",\"content\":\"{\\\"content\\\":\\\"forbidden\\\"}\"}")
if [ "$(printf '%s' "$DENIED" | jq -r '.nextCode // 0')" = '1' ] \
  && [ "$(printf '%s' "$DENIED" | jq -r '.errCode // 0')" != '0' ]; then
  pass 'OpenIM before-send callback rejects a pair without paid entitlement'
else
  fail 'OpenIM callback did not fail closed'
fi

DB_COUNT=0
CALLBACK_COUNT=0
for _ in $(seq 1 10); do
  read -r DB_COUNT CALLBACK_COUNT <<EOF
$(docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -uroot -Nse \
  "SELECT COUNT(*),SUM(openim_server_msg_id<>'') FROM askxuan_booking.booking_chat_message WHERE booking_id='$BOOKING_ID' AND status='sent';" 2>/dev/null)
EOF
  [ "$DB_COUNT" = '2' ] && [ "$CALLBACK_COUNT" = '2' ] && break
  sleep 1
done
if [ "$DB_COUNT" = '2' ] && [ "$CALLBACK_COUNT" = '2' ]; then
  pass 'after-send callbacks persist exactly two messages with OpenIM server IDs'
else
  fail "expected 2 callback-confirmed messages, got rows=$DB_COUNT callbacks=$CALLBACK_COUNT"
fi

for action in confirm start complete; do
  TRANSITION=$(curl -fsS --max-time 10 -X PUT "$GATEWAY/api/v1/admin/masters/bookings/$BOOKING_ID/$action" \
    -H "Authorization: Bearer $MASTER_TOKEN" -H 'Content-Type: application/json' -d '{"remark":"端到端验收"}')
  if [ "$(printf '%s' "$TRANSITION" | jq -r '.code // -1')" = '0' ]; then
    pass "master booking transition: $action"
  else
    fail "master booking transition failed: $action"
  fi
done

REVIEW=$(curl -fsS --max-time 10 -X POST "$GATEWAY/api/v1/bookings/$BOOKING_ID/review" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"rating":5,"content":"端到端平台分账与付费聊天验收通过","images":[]}')
if [ "$(printf '%s' "$REVIEW" | jq -r '.code // -1')" = '0' ]; then
  pass 'customer review closes the fulfilled booking'
else
  fail 'customer could not close the booking with a review'
fi

LEDGER_BALANCED=0
SETTLEMENT_BALANCED=0
MASTER_NET_MATCH=0
for _ in $(seq 1 20); do
  read -r LEDGER_BALANCED SETTLEMENT_BALANCED MASTER_NET_MATCH <<EOF
$(docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -uroot -Nse "
SELECT
  (SELECT IF(COUNT(DISTINCT ft.id)=1
    AND ROUND(SUM(IF(le.direction='debit',le.amount,0)),2)=MAX(b.total_fee)
    AND ROUND(SUM(IF(le.direction='credit',le.amount,0)),2)=MAX(b.total_fee),1,0)
   FROM askxuan_booking.booking b
   JOIN askxuan_finance.finance_transaction ft ON ft.source_type='booking' AND ft.source_no=b.booking_no AND ft.event_type='booking_settlement'
   JOIN askxuan_finance.finance_ledger_entry le ON le.transaction_id=ft.id
   WHERE b.booking_no='$BOOKING_ID'),
  (SELECT IF(COUNT(*)=IF(MAX(b.merit_money)>0,2,1)
    AND ROUND(SUM(s.total_amount),2)=MAX(b.total_fee)
    AND ROUND(SUM(s.commission_amount+s.settle_amount),2)=MAX(b.total_fee),1,0)
   FROM askxuan_booking.booking b
   JOIN askxuan_finance.settlement s ON s.source_type='booking' AND s.source_no=b.booking_no
   WHERE b.booking_no='$BOOKING_ID'),
  (SELECT IF(COUNT(*)=1 AND ROUND(MAX(e.amount),2)=ROUND(MAX(s.settle_amount),2),1,0)
   FROM askxuan_master.master_earning e
   JOIN askxuan_finance.settlement s ON s.source_type='booking' AND s.source_no=e.source_id
     AND s.settle_type='master' AND s.target_id=e.master_code
   WHERE e.source_type='booking' AND e.source_id='$BOOKING_ID');" 2>/dev/null || printf '0\t0\t0')
EOF
  [ "$LEDGER_BALANCED" = '1' ] && [ "$SETTLEMENT_BALANCED" = '1' ] && [ "$MASTER_NET_MATCH" = '1' ] && break
  sleep 1
done
if [ "$LEDGER_BALANCED" = '1' ]; then
  pass 'booking settlement has balanced debit and credit entries'
else
  fail 'booking settlement ledger is missing or unbalanced'
fi
if [ "$SETTLEMENT_BALANCED" = '1' ]; then
  pass 'platform commission plus temple and master payables equals the paid total'
else
  fail 'platform split does not reconcile to the paid total'
fi
if [ "$MASTER_NET_MATCH" = '1' ]; then
  pass 'master workspace receives only the platform-calculated net share'
else
  fail 'master earning does not match the platform settlement'
fi

printf 'Paid booking chat result: %d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
