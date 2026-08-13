#!/usr/bin/env bash
# Independent paid-consultation acceptance: quote, mock pay, platform ledger,
# split, entitlement and two-way OpenIM-backed chat.

set -euo pipefail

GATEWAY="${GATEWAY:-http://127.0.0.1:8080}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-root123}"
CUSTOMER_PHONE="${CUSTOMER_PHONE:-13800138000}"
CUSTOMER_PASSWORD="${CUSTOMER_PASSWORD:-123456}"
MASTER_ACCOUNT="${MASTER_ACCOUNT:-zhihai}"
MASTER_PASSWORD="${MASTER_PASSWORD:-123456}"
MASTER_CODE="${MASTER_CODE:-M001}"

PASS=0
FAIL=0
REQUEST_ID="consult-accept-$(date +%s)-$RANDOM"

pass() { PASS=$((PASS + 1)); printf '[PASS] %s\n' "$1"; }
fail() { FAIL=$((FAIL + 1)); printf '[FAIL] %s\n' "$1"; }

command -v jq >/dev/null 2>&1 || { printf 'jq is required\n' >&2; exit 1; }

printf '===== Independent paid consultation acceptance =====\n'

CUSTOMER_LOGIN=$(curl -fsS --max-time 10 -X POST "$GATEWAY/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"phone\":\"$CUSTOMER_PHONE\",\"password\":\"$CUSTOMER_PASSWORD\"}")
CUSTOMER_TOKEN=$(printf '%s' "$CUSTOMER_LOGIN" | jq -r '.data.accessToken // empty')
CUSTOMER_IM_TOKEN=$(printf '%s' "$CUSTOMER_LOGIN" | jq -r '.data.imToken // empty')
if [ -n "$CUSTOMER_TOKEN" ] && [ -n "$CUSTOMER_IM_TOKEN" ]; then
  pass 'customer login returns API and OpenIM tokens'
else
  fail 'customer login contract mismatch'
fi

MASTER_LOGIN=$(curl -fsS --max-time 10 -X POST "$GATEWAY/api/v1/auth/admin/login" \
  -H 'Content-Type: application/json' \
  -d "{\"account\":\"$MASTER_ACCOUNT\",\"password\":\"$MASTER_PASSWORD\"}")
MASTER_TOKEN=$(printf '%s' "$MASTER_LOGIN" | jq -r '.data.accessToken // empty')
MASTER_IM_TOKEN=$(printf '%s' "$MASTER_LOGIN" | jq -r '.data.imToken // empty')
if [ -n "$MASTER_TOKEN" ] && [ -n "$MASTER_IM_TOKEN" ]; then
  pass 'master login returns API and OpenIM tokens'
else
  fail 'master login contract mismatch'
fi

QUOTE=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/consultations/quote?masterId=$MASTER_CODE")
FEE=$(printf '%s' "$QUOTE" | jq -r '.data.consultFee // 0')
if [ "$(printf '%s' "$QUOTE" | jq -r '.data.enabled // false')" = 'true' ] \
  && awk "BEGIN { exit !($FEE > 0) }"; then
  pass 'public quote returns an enabled server-authoritative fee'
else
  fail 'consultation quote is unavailable or invalid'
fi

BOOKING_COUNT_BEFORE=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/bookings?page=1&size=1" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" | jq -r '.data.total // 0')
CREATE_PAYLOAD=$(jq -nc --arg requestId "$REQUEST_ID" --arg masterId "$MASTER_CODE" \
  '{requestId:$requestId,masterId:$masterId,question:"独立即时咨询闭环验收"}')
CREATE=$(curl -fsS --max-time 20 -X POST "$GATEWAY/api/v1/consultations" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" -H 'Content-Type: application/json' -d "$CREATE_PAYLOAD")
CONSULTATION_ID=$(printf '%s' "$CREATE" | jq -r '.data.id // empty')
if [ -n "$CONSULTATION_ID" ] \
  && [ "$(printf '%s' "$CREATE" | jq -r '.data.status // empty')" = 'active' ] \
  && [ "$(printf '%s' "$CREATE" | jq -r '.data.paymentStatus // empty')" = 'success' ] \
  && [ "$(printf '%s' "$CREATE" | jq -r '.data.simulated // false')" = 'true' ]; then
  pass 'mock payment activates a standalone consultation'
else
  fail 'standalone consultation was not activated'
fi

DUPLICATE=$(curl -fsS --max-time 20 -X POST "$GATEWAY/api/v1/consultations" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" -H 'Content-Type: application/json' -d "$CREATE_PAYLOAD")
RETRY=$(curl -fsS --max-time 20 -X POST "$GATEWAY/api/v1/consultations/$CONSULTATION_ID/pay" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN")
if [ "$(printf '%s' "$DUPLICATE" | jq -r '.data.id // empty')" = "$CONSULTATION_ID" ] \
  && [ "$(printf '%s' "$RETRY" | jq -r '.data.id // empty')" = "$CONSULTATION_ID" ]; then
  pass 'consultation creation and payment retry are idempotent'
else
  fail 'consultation idempotency mismatch'
fi

BOOKING_COUNT_AFTER=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/bookings?page=1&size=1" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" | jq -r '.data.total // 0')
if [ "$BOOKING_COUNT_BEFORE" = "$BOOKING_COUNT_AFTER" ]; then
  pass 'consultation payment does not create a temple-service booking'
else
  fail 'standalone consultation unexpectedly changed booking count'
fi

LEDGER_OK=0
for _ in $(seq 1 20); do
  LEDGER_OK=$(docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -uroot -Nse "
    SELECT IF(
      COUNT(DISTINCT ft.event_type)=2
      AND ROUND(SUM(IF(le.direction='debit',le.amount,0)),2)=ROUND(SUM(IF(le.direction='credit',le.amount,0)),2)
      AND COUNT(DISTINCT s.id)=1
      AND ROUND(MAX(s.commission_amount+s.settle_amount),2)=ROUND(MAX(c.consult_fee),2),1,0)
    FROM askxuan_booking.consultation_order c
    JOIN askxuan_finance.finance_transaction ft ON ft.source_type='consultation' AND ft.source_no=c.order_no
    JOIN askxuan_finance.finance_ledger_entry le ON le.transaction_id=ft.id
    LEFT JOIN askxuan_finance.settlement s ON s.source_type='consultation' AND s.source_no=c.order_no AND s.settle_type='master'
    WHERE c.order_no='$CONSULTATION_ID';" 2>/dev/null || printf 0)
  [ "$LEDGER_OK" = '1' ] && break
  sleep 1
done
if [ "$LEDGER_OK" = '1' ]; then
  pass 'platform receipt, balanced split and master payable reconcile to the fee'
else
  fail 'consultation platform ledger or split is missing/unbalanced'
fi

CUSTOMER_CHATS=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/chats?page=1&size=50" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN")
MASTER_CHATS=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/chats?page=1&size=50" \
  -H "Authorization: Bearer $MASTER_TOKEN")
if printf '%s' "$CUSTOMER_CHATS" | jq -e --arg id "$CONSULTATION_ID" \
  '.data.list[] | select(.conversationId==$id and .sourceType=="consultation" and .canChat==true)' >/dev/null \
  && printf '%s' "$MASTER_CHATS" | jq -e --arg id "$CONSULTATION_ID" \
  '.data.list[] | select(.conversationId==$id and .sourceType=="consultation" and .canChat==true)' >/dev/null; then
  pass 'the paid consultation appears in both clients'
else
  fail 'paid consultation visibility differs between customer and master'
fi

CUSTOMER_MARKER="customer_consult_$(date +%s)_$RANDOM"
CUSTOMER_SEND=$(curl -fsS --max-time 15 -X POST "$GATEWAY/api/v1/chats/$CONSULTATION_ID/messages" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"clientMessageId\":\"ios-$CUSTOMER_MARKER\",\"content\":\"$CUSTOMER_MARKER\"}")
if [ "$(printf '%s' "$CUSTOMER_SEND" | jq -r '.data.status // empty')" = 'sent' ]; then
  pass 'customer consultation message is delivered through OpenIM'
else
  fail 'customer consultation message delivery failed'
fi

MASTER_HISTORY=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/chats/$CONSULTATION_ID/messages?page=1&size=100" \
  -H "Authorization: Bearer $MASTER_TOKEN")
if printf '%s' "$MASTER_HISTORY" | jq -e --arg marker "$CUSTOMER_MARKER" \
  '.data.list[] | select(.content==$marker and .senderType=="customer")' >/dev/null; then
  pass 'master reads the customer consultation message'
else
  fail 'master cannot read the customer consultation message'
fi

MASTER_MARKER="master_consult_$(date +%s)_$RANDOM"
MASTER_SEND=$(curl -fsS --max-time 15 -X POST "$GATEWAY/api/v1/chats/$CONSULTATION_ID/messages" \
  -H "Authorization: Bearer $MASTER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"clientMessageId\":\"ios-$MASTER_MARKER\",\"content\":\"$MASTER_MARKER\"}")
if [ "$(printf '%s' "$MASTER_SEND" | jq -r '.data.status // empty')" = 'sent' ]; then
  pass 'master consultation reply is delivered through OpenIM'
else
  fail 'master consultation reply delivery failed'
fi

CUSTOMER_HISTORY=$(curl -fsS --max-time 10 "$GATEWAY/api/v1/chats/$CONSULTATION_ID/messages?page=1&size=100" \
  -H "Authorization: Bearer $CUSTOMER_TOKEN")
if printf '%s' "$CUSTOMER_HISTORY" | jq -e --arg marker "$MASTER_MARKER" \
  '.data.list[] | select(.content==$marker and .senderType=="master")' >/dev/null; then
  pass 'customer reads the master consultation reply'
else
  fail 'customer cannot read the master consultation reply'
fi

CALLBACK_COUNT=0
for _ in $(seq 1 10); do
  CALLBACK_COUNT=$(docker exec -e MYSQL_PWD="$MYSQL_ROOT_PASSWORD" askxuan-mysql mysql -uroot -Nse \
    "SELECT COUNT(*) FROM askxuan_booking.booking_chat_message WHERE booking_id='$CONSULTATION_ID' AND source_type='consultation' AND status='sent' AND openim_server_msg_id<>'';" 2>/dev/null || printf 0)
  [ "$CALLBACK_COUNT" = '2' ] && break
  sleep 1
done
if [ "$CALLBACK_COUNT" = '2' ]; then
  pass 'OpenIM callbacks persisted both consultation messages'
else
  fail "expected 2 callback-confirmed messages, got $CALLBACK_COUNT"
fi

printf '\nResult: %d passed, %d failed; consultation=%s\n' "$PASS" "$FAIL" "$CONSULTATION_ID"
[ "$FAIL" -eq 0 ]
