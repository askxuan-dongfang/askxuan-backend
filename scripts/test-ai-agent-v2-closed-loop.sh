#!/usr/bin/env bash
set -euo pipefail
trap 'printf "[FAIL] AI agent v2 acceptance stopped at line %s\n" "$LINENO" >&2' ERR

BASE_URL="${BASE_URL:-http://127.0.0.1:8080/api/v1}"
EXPECTED_PROVIDER="${EXPECTED_PROVIDER:-openai_compatible}"

pass() { printf '[PASS] %s\n' "$1"; }

login="$(curl -fsS -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","password":"123456"}')"
token="$(jq -r '.data.accessToken // empty' <<<"$login")"
user_id="$(jq -r '.data.userInfo.userId // empty' <<<"$login")"
[[ -n "$token" && -n "$user_id" ]]
auth="Authorization: Bearer $token"

session="$(curl -fsS -X POST "$BASE_URL/ai/sessions" -H "$auth" -H 'Content-Type: application/json' \
  -d "{\"userId\":\"$user_id\",\"skillCode\":\"auto\",\"question\":\"请用塔罗看看下一周工作重点\",\"inputs\":{\"spread\":\"single\"}}")"
[[ "$(jq -r '.code' <<<"$session")" == "0" ]]
[[ "$(jq -r '.data.skillCode' <<<"$session")" == "tarot" ]]
session_id="$(jq -r '.data.id' <<<"$session")"
message_id="$(jq -r '.data.messageId' <<<"$session")"
pass 'deterministic auto routing selected tarot'

stream="$(curl -fsSN --max-time 75 "$BASE_URL/ai/sessions/$session_id/messages/$message_id/stream" -H "$auth")"
grep -q '^event: stage' <<<"$stream"
grep -q '^event: done' <<<"$stream"
grep -q "\"provider\":\"$EXPECTED_PROVIDER\"" <<<"$stream"
pass 'SSE exposed safe stages and completed with the real provider'

trace="$(curl -fsS "$BASE_URL/ai/sessions/$session_id/messages/$message_id/trace" -H "$auth")"
[[ "$(jq -r '.code' <<<"$trace")" == "0" ]]
[[ "$(jq -r '.data.status' <<<"$trace")" == "completed" ]]
[[ "$(jq -r '.data.selectionMode' <<<"$trace")" == "auto" ]]
[[ "$(jq -r '.data.skillVersion' <<<"$trace")" == "2.0.0" ]]
[[ "$(jq -r '.data.tools | length' <<<"$trace")" == "1" ]]
[[ "$(jq -r '.data.tools[0].server' <<<"$trace")" == "taibu" ]]
[[ "$(jq -r '.data.tools[0].tool' <<<"$trace")" == "tarot" ]]
[[ "$(jq -r '.data.tools[0].status' <<<"$trace")" == "completed" ]]
[[ "$(jq -r '.data.tools[0].arguments.question' <<<"$trace")" == "[已提供]" ]]
[[ "$(jq -r '.data | has("reasoningContent") or has("chainOfThought")' <<<"$trace")" == "false" ]]
pass 'trace persisted versioned and redacted MCP audit without chain of thought'

messages="$(curl -fsS "$BASE_URL/ai/sessions/$session_id/messages?userId=$user_id" -H "$auth")"
assistant="$(jq -c --argjson id "$message_id" '[.data.list[] | select(.id == $id and .role == "assistant")][0]' <<<"$messages")"
[[ "$(jq -r '.status' <<<"$assistant")" == "completed" ]]
[[ "$(jq -r '.provider' <<<"$assistant")" == "$EXPECTED_PROVIDER" ]]
[[ "$(jq -r '.model' <<<"$assistant")" == deepseek-* ]]
[[ "$(jq -r '.costMicros' <<<"$assistant")" -gt 0 ]]
[[ "$(jq -r '.content | length' <<<"$assistant")" -gt 20 ]]
pass 'assistant response persisted model tokens and non-zero cost'

echo "PASS AI agent v2 closed loop: sessionId=$session_id route=tarot tool=taibu provider=$EXPECTED_PROVIDER"
