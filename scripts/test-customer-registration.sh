#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080/api/v1}"
MOBILE="${MOBILE:-13900000028}"
NICKNAME="${NICKNAME:-注册演示用户}"
PASS=0
TOTAL=11

require_code_zero() {
  local label="$1"
  local body="$2"
  if [ "$(printf '%s' "$body" | jq -r '.code')" = "0" ]; then
    PASS=$((PASS + 1))
    printf '[PASS] %s\n' "$label"
  else
    printf '[FAIL] %s: %s\n' "$label" "$body" >&2
    exit 1
  fi
}

command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

register_body=$(curl -ksS -X POST "$BASE_URL/users/register" \
  -H 'Content-Type: application/json' \
  -d "{\"mobile\":\"$MOBILE\",\"nickname\":\"$NICKNAME\"}")
register_code=$(printf '%s' "$register_body" | jq -r '.code')
if [ "$register_code" = "0" ]; then
  require_code_zero "手机号免真实验证码注册" "$register_body"
elif [ "$register_code" = "40901" ]; then
  PASS=$((PASS + 1))
  printf '[PASS] 手机号免真实验证码注册（账号已存在，继续回归）\n'
else
  printf '[FAIL] 手机号免真实验证码注册: %s\n' "$register_body" >&2
  exit 1
fi

login_body=$(curl -ksS -X POST "$BASE_URL/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"phone\":\"$MOBILE\",\"code\":\"1234\"}")
token=$(printf '%s' "$login_body" | jq -r '.data.accessToken // empty')
im_token=$(printf '%s' "$login_body" | jq -r '.data.imToken // empty')
user_id=$(printf '%s' "$login_body" | jq -r '.data.userInfo.userId // empty')
if [ -n "$token" ] && [ -n "$im_token" ]; then
  PASS=$((PASS + 1))
  printf '[PASS] 自动登录 JWT 与 OpenIM Token\n'
else
  printf '[FAIL] 自动登录凭据不完整: %s\n' "$login_body" >&2
  exit 1
fi

auth=(-H "Authorization: Bearer $token")
paths=(
  "users/profile"
  "users/addresses"
  "bookings?page=1&size=20"
  "orders?page=1&size=20"
  "marketing/my-coupons?page=1&size=20"
  "favorites/temples?page=1&size=20"
  "favorites/products?page=1&size=20"
  "bookings/chats?page=1&size=20"
  "messages/list?userId=$user_id&page=1&size=20"
)
labels=("用户资料" "收货地址" "预约订单" "商城订单" "优惠券" "寺院收藏" "商品收藏" "付费会话" "站内消息")

for i in "${!paths[@]}"; do
  body=$(curl -ksS "${auth[@]}" "$BASE_URL/${paths[$i]}")
  require_code_zero "${labels[$i]}空态" "$body"
done

printf '注册闭环验收：%d/%d 通过\n' "$PASS" "$TOTAL"
