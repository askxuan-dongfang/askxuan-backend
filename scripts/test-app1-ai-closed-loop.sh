#!/usr/bin/env bash
set -euo pipefail
trap 'printf "[FAIL] AI acceptance stopped at line %s\n" "$LINENO" >&2' ERR

BASE_URL="${BASE_URL:-http://127.0.0.1:8080/api/v1}"
EXPECTED_PROVIDER="${EXPECTED_PROVIDER:-mock}"

pass() { printf '[PASS] %s\n' "$1"; }

login="$(curl -fsS -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","code":"1234"}')"
token="$(jq -r '.data.accessToken' <<<"$login")"
user_id="$(jq -r '.data.userInfo.userId | tostring' <<<"$login")"
auth="Authorization: Bearer $token"

skills="$(curl -fsS "$BASE_URL/ai/skills" -H "$auth")"
[[ "$(jq -r '.code' <<<"$skills")" == "0" ]]
[[ "$(jq '.data.list | length' <<<"$skills")" -ge 8 ]]
[[ "$(jq '[.data.list[] | select(.code == "bazi")][0].inputSchema.fields | length' <<<"$skills")" -ge 5 ]]
[[ "$(jq -r '[.data.list[] | select(.code == "bazi")][0].inputSchema.fields[0].label' <<<"$skills")" == "出生日期" ]]
[[ "$(jq -r '[.data.list[] | select(.code == "tarot")][0].inputSchema.fields[0].options[0].label' <<<"$skills")" == "单牌" ]]
[[ "$(jq '[.data.list[] | has("promptTemplate") or has("toolConfig")] | any' <<<"$skills")" == "false" ]]
pass 'dynamic skills expose UTF-8 schemas without private prompts or tool config'

usage_before="$(curl -fsS "$BASE_URL/ai/usage" -H "$auth")"
daily_before="$(jq -r '.data.dailyRequests' <<<"$usage_before")"

session="$(curl -fsS -X POST "$BASE_URL/ai/sessions" -H "$auth" -H 'Content-Type: application/json' \
  -d "{\"userId\":\"$user_id\",\"question\":\"请给我一个平静审慎的今日建议\"}")"
[[ "$(jq -r '.code' <<<"$session")" == "0" ]]
session_id="$(jq -r '.data.id // empty' <<<"$session")"
message_id="$(jq -r '.data.messageId // empty' <<<"$session")"
[[ -n "$session_id" ]]
[[ -n "$message_id" ]]
[[ "$(jq -r '.data.skillCode' <<<"$session")" == "general" ]]

stream="$(curl -fsSN --max-time 75 "$BASE_URL/ai/sessions/$session_id/messages/$message_id/stream" -H "$auth")"
grep -q '^event: done' <<<"$stream"
grep -q "\"provider\":\"$EXPECTED_PROVIDER\"" <<<"$stream"
if [[ "$EXPECTED_PROVIDER" == "mock" ]]; then
  grep -q '\[本地开发模拟\]' <<<"$stream"
else
  ! grep -q '\[本地开发模拟\]' <<<"$stream"
fi
pass 'SSE streams the assistant response and final provider metadata'

poll_completed() {
  local response status
  for _ in $(seq 1 30); do
    response="$(curl -fsS "$BASE_URL/ai/sessions/$session_id/messages?userId=$user_id" -H "$auth")"
    [[ "$(jq -r '.code' <<<"$response")" == "0" ]]
    status="$(jq -r '[.data.list[] | select(.role == "assistant")][-1].status // empty' <<<"$response")"
    if [[ "$status" == "completed" ]]; then
      printf '%s' "$response"
      return 0
    fi
    [[ "$status" != "failed" ]] || return 1
    sleep 1
  done
  return 1
}

messages="$(poll_completed)"
[[ "$(jq '[.data.list[] | select(.role == "user")] | length' <<<"$messages")" -ge 1 ]]
[[ -n "$(jq -r '[.data.list[] | select(.role == "assistant")][-1].content' <<<"$messages")" ]]
pass 'conversation history persists by authenticated user'

forbidden="$(curl -sS "$BASE_URL/ai/sessions/$session_id?userId=999" -H "$auth")"
[[ "$(jq -r '.code' <<<"$forbidden")" == "40301" ]]
pass 'JWT ownership rejects cross-user session access'

invalid_bazi="$(curl -sS -X POST "$BASE_URL/ai/sessions" -H "$auth" -H 'Content-Type: application/json' \
  -d "{\"skillCode\":\"bazi\",\"question\":\"请排盘\"}")"
[[ "$(jq -r '.code' <<<"$invalid_bazi")" == "40003" ]]
pass 'structured skills reject missing birth time and place inputs'

sent="$(curl -fsS -X POST "$BASE_URL/ai/sessions/$session_id/messages" -H "$auth" -H 'Content-Type: application/json' \
  -d "{\"userId\":\"$user_id\",\"content\":\"再给我一句可执行的提醒\"}")"
[[ "$(jq -r '.data.status' <<<"$sent")" == "pending" ]]
poll_completed >/dev/null

usage_after="$(curl -fsS "$BASE_URL/ai/usage" -H "$auth")"
daily_after="$(jq -r '.data.dailyRequests' <<<"$usage_after")"
[[ "$daily_after" -ge $((daily_before + 2)) ]]
pass 'daily quota and usage counters include both accepted generations'

echo "PASS AI agent closed loop: sessionId=$session_id skills=dynamic stream=sse ownership=ok quota=ok"
