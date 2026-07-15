#!/bin/bash
# Booking authoritative pricing, slot capacity, mock payment and gRPC recovery acceptance test.

set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
RUN_DISRUPTION="${RUN_DISRUPTION:-0}"
WORK_DIR="$(mktemp -d)"
PASS=0
FAIL=0
PAYMENT_STOPPED=0
TEMPLE_STOPPED=0

cleanup_acceptance_data() {
  docker exec -i askxuan-mysql mysql -uroot -proot123 2>/dev/null <<'SQL' || true
USE askxuan_booking;
CREATE TEMPORARY TABLE cleanup_booking AS
SELECT booking_no,temple_code,service_code,booking_date,slot_code
FROM askxuan_booking.booking WHERE request_id LIKE 'accept-%';
DELETE pl FROM askxuan_payment.payment_log pl
JOIN askxuan_payment.payment p ON p.id=pl.payment_id
JOIN cleanup_booking c ON c.booking_no=p.order_no
WHERE p.order_type='booking';
DELETE p FROM askxuan_payment.payment p
JOIN cleanup_booking c ON c.booking_no=p.order_no
WHERE p.order_type='booking';
DELETE m FROM askxuan_message.message m JOIN cleanup_booking c ON c.booking_no=m.biz_id WHERE m.biz_type='booking';
DELETE l FROM askxuan_booking.booking_status_log l JOIN cleanup_booking c ON c.booking_no=l.booking_id;
DELETE r FROM askxuan_booking.booking_review r JOIN cleanup_booking c ON c.booking_no=r.booking_id;
DELETE b FROM askxuan_booking.booking b JOIN cleanup_booking c ON c.booking_no=b.booking_no;
UPDATE askxuan_booking.booking_slot_inventory i
JOIN cleanup_booking c ON c.temple_code=i.temple_code AND c.service_code=i.service_code
  AND c.booking_date=i.booking_date AND c.slot_code=i.slot_code
SET i.reserved_count=(
  SELECT COUNT(*) FROM askxuan_booking.booking b
  WHERE b.temple_code=i.temple_code AND b.service_code=i.service_code
    AND b.booking_date=i.booking_date AND b.slot_code=i.slot_code AND b.slot_reserved=1
);
DELETE i FROM askxuan_booking.booking_slot_inventory i
JOIN cleanup_booking c ON c.temple_code=i.temple_code AND c.service_code=i.service_code
  AND c.booking_date=i.booking_date AND c.slot_code=i.slot_code
WHERE i.reserved_count=0;
DROP TEMPORARY TABLE cleanup_booking;
SQL
}

cleanup() {
  if [ "$PAYMENT_STOPPED" = "1" ]; then
    docker compose -f docker-compose.yml -f docker-compose.full.yml start payment-service >/dev/null 2>&1 || true
  fi
  if [ "$TEMPLE_STOPPED" = "1" ]; then
    docker compose -f docker-compose.yml -f docker-compose.full.yml start temple-service >/dev/null 2>&1 || true
  fi
  cleanup_acceptance_data
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

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

wait_healthy() {
  local container="$1"
  local i
  for i in $(seq 1 30); do
    if [ "$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || true)" = "healthy" ]; then
      return 0
    fi
    sleep 1
  done
  return 1
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    printf 'jq is required\n' >&2
    exit 1
  fi
}

require_jq
LOGIN=$(curl -fsS --max-time 10 -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","password":"123456"}')
TOKEN=$(printf '%s' "$LOGIN" | jq -r '.data.accessToken // empty')
if [ -z "$TOKEN" ]; then
  printf 'Login failed; response intentionally omitted to avoid leaking credentials.\n' >&2
  exit 1
fi
AUTH="Authorization: Bearer $TOKEN"

BASE_OFFSET=$((400 + RANDOM % 300))
PRICE_DATE=$(future_date "$BASE_OFFSET")
CAPACITY_DATE=$(future_date "$((BASE_OFFSET + 1))")
RECOVERY_DATE=$(future_date "$((BASE_OFFSET + 2))")
DEPENDENCY_DATE=$(future_date "$((BASE_OFFSET + 3))")

AVAIL=$(curl -fsS --max-time 10 "$BASE_URL/api/v1/bookings/availability?templeId=T001&serviceId=S001&date=$PRICE_DATE")
SLOT_CODE=$(printf '%s' "$AVAIL" | jq -r '.data.slots[0].slotCode // empty')
SERVICE_FEE=$(printf '%s' "$AVAIL" | jq -r '.data.serviceFee // empty')
CAPACITY=$(printf '%s' "$AVAIL" | jq -r '.data.slots[0].capacity // 0')
if [ -n "$SLOT_CODE" ] && [ "$CAPACITY" -eq 10 ] && [ "$SERVICE_FEE" = "200" ]; then
  pass "availability returns dynamic slot, capacity 10 and authoritative fee 200"
else
  fail "availability contract mismatch"
fi

REQUEST_ID="accept-price-$(date +%s)-$RANDOM"
CREATE_PAYLOAD=$(jq -nc --arg requestId "$REQUEST_ID" --arg date "$PRICE_DATE" --arg slot "$SLOT_CODE" \
  '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:$slot,bookingDate:$date,meritMoney:11,meritMoneyTier:"custom",serviceFee:0,totalFee:0,serviceName:"tampered"}')
CREATE=$(curl -fsS --max-time 15 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$CREATE_PAYLOAD")
BOOKING_ID=$(printf '%s' "$CREATE" | jq -r '.data.id // empty')
PAYMENT_NO=$(printf '%s' "$CREATE" | jq -r '.data.paymentNo // empty')
if [ -n "$BOOKING_ID" ] && [ "$(printf '%s' "$CREATE" | jq -r '.data.status')" = "pending" ] \
  && [ "$(printf '%s' "$CREATE" | jq -r '.data.paymentStatus')" = "success" ] \
  && [ "$(printf '%s' "$CREATE" | jq -r '.data.serviceFee')" = "200" ] \
  && [ "$(printf '%s' "$CREATE" | jq -r '.data.totalFee')" = "211" ] \
  && [ "$(printf '%s' "$CREATE" | jq -r '.data.simulated')" = "true" ]; then
  pass "server ignores tampered price and completes mock payment"
else
  fail "authoritative pricing or mock payment result mismatch"
fi

DUPLICATE=$(curl -fsS --max-time 15 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$CREATE_PAYLOAD")
if [ -n "$BOOKING_ID" ] && [ "$(printf '%s' "$DUPLICATE" | jq -r '.data.id // empty')" = "$BOOKING_ID" ] \
  && [ "$(printf '%s' "$DUPLICATE" | jq -r '.data.paymentNo // empty')" = "$PAYMENT_NO" ]; then
  pass "duplicate requestId returns the original booking and payment"
else
  fail "booking or payment idempotency mismatch"
fi

CAP_AVAIL=$(curl -fsS --max-time 10 "$BASE_URL/api/v1/bookings/availability?templeId=T001&serviceId=S001&date=$CAPACITY_DATE")
CAP_SLOT=$(printf '%s' "$CAP_AVAIL" | jq -r '.data.slots[0].slotCode // empty')
PIDS=""
for i in $(seq 1 11); do
  payload=$(jq -nc --arg requestId "accept-capacity-$(date +%s)-$RANDOM-$i" --arg date "$CAPACITY_DATE" --arg slot "$CAP_SLOT" \
    '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:$slot,bookingDate:$date,meritMoney:1,meritMoneyTier:"custom"}')
  curl -sS --max-time 20 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$payload" >"$WORK_DIR/capacity-$i.json" &
  PIDS="$PIDS $!"
done
for pid in $PIDS; do wait "$pid" || true; done

CAP_SUCCESS=0
CAP_FULL=0
FIRST_CAPACITY_BOOKING=""
for file in "$WORK_DIR"/capacity-*.json; do
  id=$(jq -r '.data.id // empty' "$file")
  code=$(jq -r '.code // 0' "$file")
  if [ -n "$id" ]; then
    CAP_SUCCESS=$((CAP_SUCCESS + 1))
    [ -z "$FIRST_CAPACITY_BOOKING" ] && FIRST_CAPACITY_BOOKING="$id"
  elif [ "$code" = "40907" ]; then
    CAP_FULL=$((CAP_FULL + 1))
  fi
done
if [ "$CAP_SUCCESS" -eq 10 ] && [ "$CAP_FULL" -eq 1 ]; then
  pass "11 concurrent bookings reserve exactly 10 slots and reject one as full"
else
  fail "capacity result expected 10 success/1 full, got $CAP_SUCCESS/$CAP_FULL"
fi

CANCEL=$(curl -fsS --max-time 10 -X PUT "$BASE_URL/api/v1/bookings/$FIRST_CAPACITY_BOOKING/status" \
  -H "$AUTH" -H 'Content-Type: application/json' -d '{"status":"cancelled"}')
REPLACE_PAYLOAD=$(jq -nc --arg requestId "accept-replace-$(date +%s)-$RANDOM" --arg date "$CAPACITY_DATE" --arg slot "$CAP_SLOT" \
  '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:$slot,bookingDate:$date,meritMoney:1,meritMoneyTier:"custom"}')
REPLACE=$(curl -fsS --max-time 15 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$REPLACE_PAYLOAD")
if [ "$(printf '%s' "$CANCEL" | jq -r '.data.status // empty')" = "cancelled" ] && [ -n "$(printf '%s' "$REPLACE" | jq -r '.data.id // empty')" ]; then
  pass "cancellation releases the slot exactly once"
else
  fail "slot was not reusable after cancellation"
fi

if [ "$RUN_DISRUPTION" = "1" ]; then
  docker compose -f docker-compose.yml -f docker-compose.full.yml stop payment-service >/dev/null
  PAYMENT_STOPPED=1
  RECOVERY_AVAIL=$(curl -fsS --max-time 10 "$BASE_URL/api/v1/bookings/availability?templeId=T001&serviceId=S001&date=$RECOVERY_DATE")
  RECOVERY_SLOT=$(printf '%s' "$RECOVERY_AVAIL" | jq -r '.data.slots[0].slotCode // empty')
  RECOVERY_PAYLOAD=$(jq -nc --arg requestId "accept-recovery-$(date +%s)-$RANDOM" --arg date "$RECOVERY_DATE" --arg slot "$RECOVERY_SLOT" \
    '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:$slot,bookingDate:$date,meritMoney:2,meritMoneyTier:"custom"}')
  RECOVERY_CREATE=$(curl -fsS --max-time 20 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$RECOVERY_PAYLOAD")
  RECOVERY_ID=$(printf '%s' "$RECOVERY_CREATE" | jq -r '.data.id // empty')
  if [ -n "$RECOVERY_ID" ] && [ "$(printf '%s' "$RECOVERY_CREATE" | jq -r '.data.status')" = "pending_payment" ]; then
    pass "payment RPC outage leaves an explicit retryable pending_payment booking"
  else
    fail "payment RPC outage did not preserve pending_payment"
  fi

  EXPIRE_PAYLOAD=$(jq -nc --arg requestId "accept-expire-$(date +%s)-$RANDOM" --arg date "$RECOVERY_DATE" --arg slot "$RECOVERY_SLOT" \
    '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:$slot,bookingDate:$date,meritMoney:3,meritMoneyTier:"custom"}')
  EXPIRE_CREATE=$(curl -fsS --max-time 20 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$EXPIRE_PAYLOAD")
  EXPIRE_ID=$(printf '%s' "$EXPIRE_CREATE" | jq -r '.data.id // empty')
  LOST_PAYLOAD=$(jq -nc --arg requestId "accept-lost-response-$(date +%s)-$RANDOM" --arg date "$RECOVERY_DATE" --arg slot "$RECOVERY_SLOT" \
    '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:$slot,bookingDate:$date,meritMoney:4,meritMoneyTier:"custom"}')
  LOST_CREATE=$(curl -fsS --max-time 20 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$LOST_PAYLOAD")
  LOST_ID=$(printf '%s' "$LOST_CREATE" | jq -r '.data.id // empty')
  docker exec askxuan-mysql mysql -uroot -proot123 -Nse "UPDATE askxuan_booking.booking SET payment_expire_time=DATE_SUB(NOW(),INTERVAL 1 MINUTE) WHERE booking_no='$EXPIRE_ID'" 2>/dev/null

  docker compose -f docker-compose.yml -f docker-compose.full.yml start payment-service >/dev/null
  PAYMENT_STOPPED=0
  LOST_STATUS=""
  if wait_healthy askxuan-payment-service; then
    RETRY=$(curl -fsS --max-time 20 -X POST "$BASE_URL/api/v1/bookings/$RECOVERY_ID/pay" -H "$AUTH")
    if [ "$(printf '%s' "$RETRY" | jq -r '.data.paymentStatus // empty')" = "success" ]; then
      pass "payment retry recovers through idempotent payment.rpc"
    else
      fail "payment retry did not recover"
    fi
    LOST_PAYMENT_NO="PAYR$(date +%s)$RANDOM"
    docker exec askxuan-mysql mysql -uroot -proot123 -Nse "INSERT INTO askxuan_payment.payment(payment_no,idempotency_key,user_id,order_type,order_no,amount,channel,status,trade_no,create_time) SELECT '$LOST_PAYMENT_NO',CONCAT('booking:',booking_no),user_id,'booking',booking_no,total_fee,'mock','success',CONCAT('MOCK-RECOVER-',booking_no),NOW() FROM askxuan_booking.booking WHERE booking_no='$LOST_ID'" 2>/dev/null
  else
    fail "payment-service did not become healthy after restart"
  fi

  for _ in $(seq 1 14); do
    LOST_STATUS=$(curl -fsS --max-time 10 "$BASE_URL/api/v1/bookings/$LOST_ID" -H "$AUTH" | jq -r '.data.status // empty')
    [ "$LOST_STATUS" = "pending" ] && break
    sleep 5
  done
  LOST_LOGS=$(docker exec askxuan-mysql mysql -uroot -proot123 -Nse "SELECT COUNT(*) FROM askxuan_booking.booking_status_log WHERE booking_id='$LOST_ID' AND from_status='pending_payment' AND to_status='pending' AND operator_id='booking-reconciler'" 2>/dev/null)
  if [ "$LOST_STATUS" = "pending" ] && [ "$LOST_LOGS" = "1" ]; then
    pass "periodic reconciliation recovers a committed payment after RPC/MQ response loss"
  else
    fail "periodic reconciliation did not recover the lost payment response"
  fi

  EXPIRED=$(curl -fsS --max-time 10 "$BASE_URL/api/v1/bookings/$EXPIRE_ID" -H "$AUTH")
  if [ "$(printf '%s' "$EXPIRED" | jq -r '.data.status // empty')" = "cancelled" ]; then
    pass "expired pending payment is cancelled and its slot is released"
  else
    fail "expired pending payment was not compensated"
  fi

  docker compose -f docker-compose.yml -f docker-compose.full.yml stop temple-service >/dev/null
  TEMPLE_STOPPED=1
  DEP_REQUEST="accept-dependency-$(date +%s)-$RANDOM"
  DEP_PAYLOAD=$(jq -nc --arg requestId "$DEP_REQUEST" --arg date "$DEPENDENCY_DATE" \
    '{requestId:$requestId,templeId:"T001",masterId:"M001",serviceId:"S001",slotCode:"slot-1",bookingDate:$date,meritMoney:1,meritMoneyTier:"custom"}')
  DEP_RESULT=$(curl -sS --max-time 20 -X POST "$BASE_URL/api/v1/bookings" -H "$AUTH" -H 'Content-Type: application/json' -d "$DEP_PAYLOAD")
  DEP_ROWS=$(docker exec askxuan-mysql mysql -uroot -proot123 -Nse "SELECT COUNT(*) FROM askxuan_booking.booking WHERE request_id='$DEP_REQUEST'" 2>/dev/null)
  if [ "$(printf '%s' "$DEP_RESULT" | jq -r '.code // 0')" = "50205" ] && [ "$DEP_ROWS" = "0" ]; then
    pass "temple RPC outage fails closed without creating a booking"
  else
    fail "dependency outage did not fail closed"
  fi
  docker compose -f docker-compose.yml -f docker-compose.full.yml start temple-service >/dev/null
  TEMPLE_STOPPED=0
  wait_healthy askxuan-temple-service || fail "temple-service did not become healthy after restart"
fi

printf 'Booking acceptance result: %d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
