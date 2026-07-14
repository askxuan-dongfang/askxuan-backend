#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${MEDIA_BASE_URL:-http://127.0.0.1:8100/api/v1}"
CALLBACK_TOKEN="${MEDIA_CALLBACK_TOKEN:-local-media-callback-token}"
OWNER_ID="${MEDIA_TEST_OWNER_ID:-media-e2e-user}"
MASTER_ID="${MEDIA_TEST_MASTER_ID:-M001}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

printf 'mock-cover' > "$TMP_DIR/cover.jpg"
printf 'mock-video-content' > "$TMP_DIR/video.mp4"

request_credential() {
  local file_name="$1" media_type="$2" content_type="$3" file_size="$4"
  local response
  response="$(curl -fsS -X POST "$BASE_URL/media/uploads/credentials" \
    -H 'Content-Type: application/json' \
    -d "{\"userId\":\"$OWNER_ID\",\"fileName\":\"$file_name\",\"mediaType\":\"$media_type\",\"contentType\":\"$content_type\",\"fileSize\":$file_size}")"
  [[ "$(jq -r '.code' <<<"$response")" == "0" ]]
  [[ "$(jq -r '.data.mediaId // empty' <<<"$response")" =~ ^[0-9]+$ ]]
  printf '%s' "$response"
}

upload_and_complete() {
  local credential="$1" file="$2" content_type="$3" cover_id="${4:-0}"
  local media_id upload_url complete_body
  media_id="$(jq -r '.data.mediaId' <<<"$credential")"
  upload_url="$(jq -r '.data.uploadUrl' <<<"$credential")"
  [[ "$media_id" =~ ^[0-9]+$ && -n "$upload_url" ]]
  if ! curl -fsS -X PUT "$upload_url" -H "Content-Type: $content_type" --data-binary "@$file" >/dev/null; then
    return 1
  fi
  complete_body="{\"userId\":\"$OWNER_ID\"}"
  if [[ "$cover_id" != "0" ]]; then
    complete_body="{\"userId\":\"$OWNER_ID\",\"coverMediaId\":$cover_id}"
  fi
  local result
  result="$(curl -fsS -X POST "$BASE_URL/media/$media_id/complete" -H 'Content-Type: application/json' -d "$complete_body")"
  [[ "$(jq -r '.code' <<<"$result")" == "0" ]]
  [[ "$(jq -r '.data.id // empty' <<<"$result")" == "$media_id" ]]
  printf '%s' "$result"
}

cover_credential="$(request_credential cover.jpg image image/jpeg "$(wc -c < "$TMP_DIR/cover.jpg" | tr -d ' ')")"
cover_result="$(upload_and_complete "$cover_credential" "$TMP_DIR/cover.jpg" image/jpeg)"
cover_id="$(jq -r '.data.id' <<<"$cover_result")"
[[ "$cover_id" =~ ^[0-9]+$ ]]

video_credential="$(request_credential video.mp4 video video/mp4 "$(wc -c < "$TMP_DIR/video.mp4" | tr -d ' ')")"
video_result="$(upload_and_complete "$video_credential" "$TMP_DIR/video.mp4" video/mp4 "$cover_id")"
video_id="$(jq -r '.data.id' <<<"$video_result")"
[[ "$video_id" =~ ^[0-9]+$ ]]
playback_url="$(jq -r '.data.playbackUrl' <<<"$video_result")"
[[ -n "$playback_url" && "$(jq -r '.data.coverMediaId' <<<"$video_result")" == "$cover_id" ]]
repeat_complete="$(curl -fsS -X POST "$BASE_URL/media/$video_id/complete" -H 'Content-Type: application/json' -d "{\"userId\":\"$OWNER_ID\",\"coverMediaId\":$cover_id}")"
[[ "$(jq -r '.data.id' <<<"$repeat_complete")" == "$video_id" ]]

wrong_callback="$(curl -sS -X POST "$BASE_URL/media/callback/transcode" \
  -H 'Content-Type: application/json' -H 'X-Media-Callback-Token: wrong' \
  -d "{\"mediaId\":$video_id,\"status\":\"ready\"}")"
[[ "$(jq -r '.code' <<<"$wrong_callback")" == "40301" ]]

for _ in 1 2; do
  callback="$(curl -fsS -X POST "$BASE_URL/media/callback/transcode" \
    -H 'Content-Type: application/json' -H "X-Media-Callback-Token: $CALLBACK_TOKEN" \
    -d "{\"mediaId\":$video_id,\"status\":\"ready\",\"duration\":8.5}")"
  [[ "$(jq -r '.code' <<<"$callback")" == "0" ]]
done

audit="$(curl -fsS -X POST "$BASE_URL/media/callback/audit" \
  -H 'Content-Type: application/json' -H "X-Media-Callback-Token: $CALLBACK_TOKEN" \
  -d "{\"mediaId\":$video_id,\"auditStatus\":\"approved\"}")"
[[ "$(jq -r '.code' <<<"$audit")" == "0" ]]

detail="$(curl -fsS "$BASE_URL/media/$video_id" -H "X-User-Id: $OWNER_ID")"
[[ "$(jq -r '.data.playbackUrl' <<<"$detail")" == "$playback_url" ]]
[[ "$(jq -r '.data.auditStatus' <<<"$detail")" == "approved" ]]
anonymous_detail="$(curl -sS "$BASE_URL/media/$video_id")"
[[ "$(jq -r '.code' <<<"$anonymous_detail")" == "40301" ]]
viewer_detail="$(curl -fsS "$BASE_URL/media/$video_id" -H 'X-User-Id: media-e2e-viewer')"
[[ "$(jq -r '.data.id' <<<"$viewer_detail")" == "$video_id" ]]

capabilities="$(curl -fsS "$BASE_URL/live/capabilities")"
[[ "$(jq -r '.data.canStart' <<<"$capabilities")" == "false" ]]

room="$(curl -fsS -X POST "$BASE_URL/live/rooms" -H 'Content-Type: application/json' \
  -d "{\"ownerId\":\"$OWNER_ID\",\"masterId\":\"$MASTER_ID\",\"title\":\"媒体闭环验证\",\"openimGroupId\":\"media-e2e-group\"}")"
room_id="$(jq -r '.data.id' <<<"$room")"
[[ "$room_id" =~ ^[0-9]+$ ]]
[[ -z "$(jq -r '.data.pushUrl' <<<"$room")" ]]
draft_view="$(curl -sS "$BASE_URL/live/rooms/$room_id" -H 'X-User-Id: media-e2e-viewer')"
[[ "$(jq -r '.code' <<<"$draft_view")" == "40301" ]]

start_result="$(curl -sS -X POST "$BASE_URL/live/rooms/$room_id/start" -H 'Content-Type: application/json' -d "{\"ownerId\":\"$OWNER_ID\"}")"
[[ "$(jq -r '.code' <<<"$start_result")" == "50320" ]]
[[ -z "$(jq -r '.data.pushUrl // empty' <<<"$start_result")" ]]

public_list="$(curl -fsS "$BASE_URL/live/rooms?masterId=$MASTER_ID")"
[[ "$(jq --arg id "$room_id" '[.data.list[] | select((.id | tostring) == $id)] | length' <<<"$public_list")" == "0" ]]

echo "PASS media/live closed loop: mediaId=$video_id roomId=$room_id"
