#!/bin/bash
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"
PASS=0
FAIL=0
STAMP="$(date +%s)"
REQUEST_ID="shop-checkout-${STAMP}"

pass() { printf '[PASS] %s\n' "$1"; PASS=$((PASS + 1)); }
fail() { printf '[FAIL] %s\n' "$1"; FAIL=$((FAIL + 1)); }

ADMIN_RESP=$(curl -sS -X POST "$BASE/api/v1/auth/admin/login" -H 'Content-Type: application/json' -d '{"account":"admin","password":"123456"}')
ADMIN_TOKEN=$(printf '%s' "$ADMIN_RESP" | jq -r '.data.accessToken // empty')
if [ -n "$ADMIN_TOKEN" ]; then pass "shop admin login"; else fail "shop admin login: $ADMIN_RESP"; exit 1; fi

PRODUCT_RESP=$(curl -sS -X POST "$BASE/api/v1/admin/products" -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d "{\"name\":\"商城闭环测试-$STAMP\",\"categoryId\":1,\"description\":\"authoritative checkout test\",\"mainImage\":\"\",\"price\":99,\"marketPrice\":128,\"stock\":5,\"tags\":\"test\",\"freightTemplateId\":0}")
PRODUCT_ID=$(printf '%s' "$PRODUCT_RESP" | jq -r '.data.id // empty')
STATUS_RESP=$(curl -sS -X PUT "$BASE/api/v1/admin/products/$PRODUCT_ID/status" -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"status":"on_shelf"}')
if [ "$(printf '%s' "$STATUS_RESP" | jq -r '.data.status // empty')" = "on_shelf" ]; then pass "create and publish product"; else fail "publish product: $STATUS_RESP"; fi

LOGIN_RESP=$(curl -sS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"phone":"13800138000","code":"1234"}')
TOKEN=$(printf '%s' "$LOGIN_RESP" | jq -r '.data.accessToken // empty')
USER_ID=$(printf '%s' "$LOGIN_RESP" | jq -r '.data.userInfo.userId // empty')
if [ -n "$TOKEN" ] && [ -n "$USER_ID" ]; then pass "customer login"; else fail "customer login: $LOGIN_RESP"; exit 1; fi

ORDER_BODY=$(jq -n --arg requestId "$REQUEST_ID" --argjson productId "$PRODUCT_ID" '{requestId:$requestId,userId:"999999",addressId:1,note:"tampered checkout",items:[{productId:$productId,skuId:0,productName:"tampered name",skuSpec:"tampered",price:0.01,quantity:2,image:"tampered"}]}')
ORDER_RESP=$(curl -sS -X POST "$BASE/api/v1/orders" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$ORDER_BODY")
ORDER_ID=$(printf '%s' "$ORDER_RESP" | jq -r '.data.id // empty')
ORDER_NO=$(printf '%s' "$ORDER_RESP" | jq -r '.data.orderNo // empty')
PAY_AMOUNT=$(printf '%s' "$ORDER_RESP" | jq -r '.data.payAmount // 0')
if [ "$PAY_AMOUNT" = "198" ] || [ "$PAY_AMOUNT" = "198.0" ]; then pass "server ignores tampered price and calculates 99 x 2"; else fail "authoritative price: $ORDER_RESP"; fi

DETAIL=$(curl -sS "$BASE/api/v1/orders/$ORDER_ID" -H "Authorization: Bearer $TOKEN")
if [ "$(printf '%s' "$DETAIL" | jq -r '.data.userId')" = "$USER_ID" ] && [ "$(printf '%s' "$DETAIL" | jq -r '.data.items[0].productName')" != "tampered name" ]; then pass "JWT identity and immutable product snapshot win"; else fail "identity or snapshot: $DETAIL"; fi

DUPLICATE=$(curl -sS -X POST "$BASE/api/v1/orders" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "$ORDER_BODY")
if [ "$(printf '%s' "$DUPLICATE" | jq -r '.data.id // empty')" = "$ORDER_ID" ]; then pass "requestId makes order creation idempotent"; else fail "order idempotency: $DUPLICATE"; fi

BAD_PAY=$(curl -sS -X POST "$BASE/api/v1/payments" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"orderType\":\"shop_order\",\"orderNo\":\"$ORDER_NO\",\"amount\":0.01,\"channel\":\"mock\",\"userId\":\"999999\"}")
if [ "$(printf '%s' "$BAD_PAY" | jq -r '.code')" != "0" ]; then pass "underpayment is rejected by order.rpc"; else fail "underpayment accepted: $BAD_PAY"; fi

PAY_RESP=$(curl -sS -X POST "$BASE/api/v1/payments" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d "{\"orderType\":\"shop_order\",\"orderNo\":\"$ORDER_NO\",\"amount\":198,\"channel\":\"mock\",\"userId\":\"999999\"}")
PAYMENT_ID=$(printf '%s' "$PAY_RESP" | jq -r '.data.id // empty')
PAYMENT=$(curl -sS "$BASE/api/v1/payments/$PAYMENT_ID" -H "Authorization: Bearer $TOKEN")
if [ "$(printf '%s' "$PAYMENT" | jq -r '.data.status // empty')" = "success" ] && [ "$(printf '%s' "$PAYMENT" | jq -r '.data.channel // empty')" = "mock" ]; then pass "development mock payment succeeds explicitly"; else fail "mock payment: $PAYMENT"; fi

ORDER_STATUS=""
for _ in $(seq 1 20); do
  ORDER_STATUS=$(curl -sS "$BASE/api/v1/orders/$ORDER_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.status // empty')
  [ "$ORDER_STATUS" = "paid" ] && break
  sleep 0.25
done
if [ "$ORDER_STATUS" = "paid" ]; then pass "payment.notify moves order to paid"; else fail "order status after payment: $ORDER_STATUS"; fi

STOCK=$(curl -sS "$BASE/api/v1/admin/products/$PRODUCT_ID" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r '.data.stock // -1')
if [ "$STOCK" = "3" ]; then pass "catalog reservation deducts stock exactly once"; else fail "stock expected 3, got $STOCK"; fi

printf '\nShop checkout closed loop: %d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
