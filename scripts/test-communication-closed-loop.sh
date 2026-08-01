#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://127.0.0.1:8080}"
REQUIRE_OPENIM="${REQUIRE_OPENIM:-1}"
PASS=0
FAIL=0

green='\033[0;32m'
red='\033[0;31m'
yellow='\033[1;33m'
nc='\033[0m'

pass() { echo -e "${green}[PASS]${nc} $1"; PASS=$((PASS+1)); }
fail() { echo -e "${red}[FAIL]${nc} $1"; FAIL=$((FAIL+1)); }
info() { echo -e "${yellow}[INFO]${nc} $1"; }

need_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "错误：需要 jq" >&2
    exit 1
  fi
}

need_jq

info "1. gateway 健康检查"
HEALTH="$(curl -sS "$BASE/api/v1/health")"
if [ "$(echo "$HEALTH" | jq -r '.code')" = "0" ]; then pass "gateway health"; else fail "$HEALTH"; fi

info "2. C端登录 + refresh + imToken"
LOGIN="$(curl -sS -X POST "$BASE/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"phone":"13800138000","code":"1234"}')"
TOKEN="$(echo "$LOGIN" | jq -r '.data.accessToken // empty')"
REFRESH="$(echo "$LOGIN" | jq -r '.data.refreshToken // empty')"
IM_TOKEN="$(echo "$LOGIN" | jq -r '.data.imToken // empty')"
USER_ID="$(echo "$LOGIN" | jq -r '.data.userInfo.userId // empty')"
if [ -n "$TOKEN" ] && [ -n "$REFRESH" ] && [ -n "$USER_ID" ]; then pass "用户登录 token"; else fail "$LOGIN"; fi
if [ "$REQUIRE_OPENIM" = "1" ]; then
  if [ -n "$IM_TOKEN" ]; then pass "OpenIM imToken 非空"; else fail "imToken 为空，请确认 OpenIM 已启动"; fi
fi
REFRESH_RESP="$(curl -sS -X POST "$BASE/api/v1/auth/refresh" -H 'Content-Type: application/json' -d "{\"refreshToken\":\"$REFRESH\"}")"
if [ -n "$(echo "$REFRESH_RESP" | jq -r '.data.accessToken // empty')" ]; then pass "refresh token"; else fail "$REFRESH_RESP"; fi

AUTH="Authorization: Bearer $TOKEN"

info "3. device token 注册/解绑"
DEVICE_TOKEN="mock-communication-customer-$USER_ID"
DT_RESP="$(curl -sS -X POST "$BASE/api/v1/messages/device-token" -H "$AUTH" -H 'Content-Type: application/json' -d "{\"userId\":\"$USER_ID\",\"clientType\":\"customer\",\"platform\":\"ios\",\"deviceToken\":\"$DEVICE_TOKEN\",\"bundleId\":\"com.dongfang.customer\",\"appVersion\":\"0.1.0\"}")"
if [ "$(echo "$DT_RESP" | jq -r '.code')" = "0" ] && [ "$(echo "$DT_RESP" | jq -r '.data.status')" = "active" ]; then pass "device token 注册"; else fail "$DT_RESP"; fi
UNBIND_RESP="$(curl -sS -X DELETE "$BASE/api/v1/messages/device-token" -H "$AUTH" -H 'Content-Type: application/json' -d "{\"userId\":\"$USER_ID\",\"deviceToken\":\"$DEVICE_TOKEN\"}")"
if [ "$(echo "$UNBIND_RESP" | jq -r '.code')" = "0" ] && [ "$(echo "$UNBIND_RESP" | jq -r '.data.status')" = "inactive" ]; then pass "device token 解绑"; else fail "$UNBIND_RESP"; fi

info "4. 旧咨询通道不可绕过付费预约"
LEGACY_SEND="$(curl -sS -X POST "$BASE/api/v1/messages/send" -H "$AUTH" -H 'Content-Type: application/json' -d "{\"conversationId\":\"M001\",\"userId\":\"$USER_ID\",\"content\":\"不应送达\"}")"
if [ "$(echo "$LEGACY_SEND" | jq -r '.code // 0')" = "40909" ]; then
  pass "旧 /messages/send 固定拒绝未绑定付费预约的咨询"
else
  fail "$LEGACY_SEND"
fi

info "5. 付费预约 + C端/法师端 + OpenIM 双向闭环"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
if GATEWAY="$BASE" "$SCRIPT_DIR/test-openim-chat.sh"; then
  pass "付费预约实时通信闭环"
else
  fail "付费预约实时通信闭环"
fi

info "6. 管理台 push logs 查询"
ADMIN_LOGIN="$(curl -sS -X POST "$BASE/api/v1/auth/admin/login" -H 'Content-Type: application/json' -d '{"account":"admin","password":"123456"}')"
ADMIN_TOKEN="$(echo "$ADMIN_LOGIN" | jq -r '.data.accessToken // empty')"
PUSH_LOGS="$(curl -sS "$BASE/api/v1/admin/messages/push-logs?page=1&size=10" -H "Authorization: Bearer $ADMIN_TOKEN")"
if [ "$(echo "$PUSH_LOGS" | jq -r '.code // -1')" = "0" ]; then
  pass "push logs 仅作业务通知查询"
else
  fail "$PUSH_LOGS"
fi

echo "================================================"
echo "通信闭环测试：$PASS 通过，$FAIL 失败"
if [ "$FAIL" -ne 0 ]; then exit 1; fi
