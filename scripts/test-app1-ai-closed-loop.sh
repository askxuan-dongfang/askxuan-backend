#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080/api/v1}"
AI_CONTAINER="${AI_CONTAINER:-askxuan-ai-service}"

login="$(curl -fsS -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","code":"1234"}')"
token="$(jq -r '.data.accessToken' <<<"$login")"
user_id="$(jq -r '.data.userInfo.userId | tostring' <<<"$login")"
auth="Authorization: Bearer $token"

session="$(curl -fsS -X POST "$BASE_URL/ai/sessions" -H "$auth" -H 'Content-Type: application/json' \
  -d "{\"userId\":\"$user_id\",\"question\":\"请给我一个平静审慎的今日建议\"}")"
[[ "$(jq -r '.code' <<<"$session")" == "0" ]]
session_id="$(jq -r '.data.id // empty' <<<"$session")"
[[ -n "$session_id" ]]
[[ "$(jq -r '.data.skillCode' <<<"$session")" == "general" ]]

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

forbidden="$(curl -sS "$BASE_URL/ai/sessions/$session_id?userId=999" -H "$auth")"
[[ "$(jq -r '.code' <<<"$forbidden")" == "40301" ]]

docker restart "$AI_CONTAINER" >/dev/null
for _ in $(seq 1 30); do
  if docker exec "$AI_CONTAINER" nc -z 127.0.0.1 8098 2>/dev/null; then
    break
  fi
  sleep 1
done

restored="$(curl -fsS "$BASE_URL/ai/sessions/$session_id?userId=$user_id" -H "$auth")"
[[ "$(jq -r '.data.session.id' <<<"$restored")" == "$session_id" ]]
[[ "$(jq '.data.messages | length' <<<"$restored")" -ge 2 ]]

sent="$(curl -fsS -X POST "$BASE_URL/ai/sessions/$session_id/messages" -H "$auth" -H 'Content-Type: application/json' \
  -d "{\"userId\":\"$user_id\",\"content\":\"再给我一句可执行的提醒\"}")"
[[ "$(jq -r '.data.status' <<<"$sent")" == "pending" ]]
poll_completed >/dev/null

echo "PASS AI closed loop: sessionId=$session_id defaultSkill=general ownership=ok restartRestore=ok"
