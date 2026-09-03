#!/usr/bin/env bash
set -euo pipefail
trap 'printf "[FAIL] AI vision acceptance stopped at line %s\n" "$LINENO" >&2' ERR

BASE_URL="${BASE_URL:-http://127.0.0.1:8080/api/v1}"
EXPECTED_PROVIDER="${EXPECTED_PROVIDER:-openai_compatible}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

pass() { printf '[PASS] %s\n' "$1"; }

login="$(curl -fsS -X POST "$BASE_URL/auth/login" -H 'Content-Type: application/json' \
  -d '{"phone":"13800138000","password":"123456"}')"
token="$(jq -r '.data.accessToken // empty' <<<"$login")"
user_id="$(jq -r '.data.userInfo.userId // empty' <<<"$login")"
[[ -n "$token" && -n "$user_id" ]]
auth="Authorization: Bearer $token"

# A valid 1x1 PNG keeps the fixture deterministic and small.
printf '%s' 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=' \
  | base64 -d >"$TMP_DIR/vision.png"
file_size="$(wc -c <"$TMP_DIR/vision.png" | tr -d ' ')"

credential="$(curl -fsS -X POST "$BASE_URL/media/uploads/credentials" \
  -H "$auth" -H 'Content-Type: application/json' \
  -d "{\"userId\":\"$user_id\",\"fileName\":\"vision.png\",\"mediaType\":\"image\",\"contentType\":\"image/png\",\"fileSize\":$file_size}")"
media_id="$(jq -r '.data.mediaId // empty' <<<"$credential")"
upload_url="$(jq -r '.data.uploadUrl // empty' <<<"$credential")"
[[ "$media_id" =~ ^[0-9]+$ && "$upload_url" == https://* ]]

curl -fsS -X PUT "$upload_url" -H 'Content-Type: image/png' --data-binary "@$TMP_DIR/vision.png" >/dev/null
completed="$(curl -fsS -X POST "$BASE_URL/media/$media_id/complete" \
  -H "$auth" -H 'Content-Type: application/json' -d "{\"userId\":\"$user_id\"}")"
playback_url="$(jq -r '.data.playbackUrl // empty' <<<"$completed")"
[[ "$playback_url" == https://* ]]
curl -fsS "$playback_url" -o "$TMP_DIR/downloaded.png"
cmp "$TMP_DIR/vision.png" "$TMP_DIR/downloaded.png"
pass 'media upload and public HTTPS image retrieval completed'

session_body="$(jq -nc --arg user "$user_id" --arg url "$playback_url" --argjson media "$media_id" '{
  userId:$user,
  skillCode:"general",
  question:"这是一张测试图片，请简要说明你看到了什么。",
  attachments:[{mediaId:$media,url:$url,contentType:"image/png"}]
}')"
session="$(curl -fsS -X POST "$BASE_URL/ai/sessions" -H "$auth" -H 'Content-Type: application/json' -d "$session_body")"
[[ "$(jq -r '.code' <<<"$session")" == "0" ]]
session_id="$(jq -r '.data.id' <<<"$session")"
message_id="$(jq -r '.data.messageId' <<<"$session")"

stream="$(curl -fsSN --max-time 90 "$BASE_URL/ai/sessions/$session_id/messages/$message_id/stream" -H "$auth")"
grep -q '^event: stage' <<<"$stream"
grep -q '^event: done' <<<"$stream"
grep -q "\"provider\":\"$EXPECTED_PROVIDER\"" <<<"$stream"
pass 'vision request completed through safe SSE stages'

messages="$(curl -fsS "$BASE_URL/ai/sessions/$session_id/messages?userId=$user_id" -H "$auth")"
assistant="$(jq -c --argjson id "$message_id" '[.data.list[] | select(.id == $id and .role == "assistant")][0]' <<<"$messages")"
[[ "$(jq -r '.status' <<<"$assistant")" == "completed" ]]
[[ "$(jq -r '.provider' <<<"$assistant")" == "$EXPECTED_PROVIDER" ]]
[[ "$(jq -r '.model' <<<"$assistant")" == *vision* ]]
[[ "$(jq -r '.tokens' <<<"$assistant")" -gt 0 ]]
[[ "$(jq -r '.costMicros' <<<"$assistant")" -gt 0 ]]

trace="$(curl -fsS "$BASE_URL/ai/sessions/$session_id/messages/$message_id/trace" -H "$auth")"
[[ "$(jq -r '.data.status' <<<"$trace")" == "completed" ]]
[[ "$(jq -r '.data | has("reasoningContent") or has("chainOfThought")' <<<"$trace")" == "false" ]]
pass 'vision model, token usage, cost, and safe audit trace persisted'

echo "PASS AI vision closed loop: sessionId=$session_id mediaId=$media_id model=$(jq -r '.model' <<<"$assistant")"
