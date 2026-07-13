#!/usr/bin/env bash
set -euo pipefail

COMMUNITY_URL="${COMMUNITY_BASE_URL:-http://127.0.0.1:8099/api/v1}"
MEDIA_URL="${MEDIA_BASE_URL:-http://127.0.0.1:8100/api/v1}"
OWNER_ID="${COMMUNITY_TEST_OWNER_ID:-community-e2e-master}"
MASTER_ID="${COMMUNITY_TEST_MASTER_ID:-M001}"
USER_ID="${COMMUNITY_TEST_USER_ID:-community-e2e-user}"
AUDITOR_ID="${COMMUNITY_TEST_AUDITOR_ID:-community-e2e-auditor}"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

printf 'community-cover' > "$TMP_DIR/cover.jpg"
printf 'community-video' > "$TMP_DIR/video.mp4"

upload_media() {
  local file="$1" file_name="$2" media_type="$3" content_type="$4" cover_id="${5:-0}"
  local credential media_id upload_url body
  credential="$(curl -fsS -X POST "$MEDIA_URL/media/uploads/credentials" -H 'Content-Type: application/json' \
    -d "{\"userId\":\"$OWNER_ID\",\"fileName\":\"$file_name\",\"mediaType\":\"$media_type\",\"contentType\":\"$content_type\",\"fileSize\":$(wc -c < "$file" | tr -d ' ')}")"
  media_id="$(jq -r '.data.mediaId' <<<"$credential")"
  upload_url="$(jq -r '.data.uploadUrl' <<<"$credential")"
  curl -fsS -X PUT "$upload_url" -H "Content-Type: $content_type" --data-binary "@$file" >/dev/null
  body="{\"userId\":\"$OWNER_ID\"}"
  [[ "$cover_id" == "0" ]] || body="{\"userId\":\"$OWNER_ID\",\"coverMediaId\":$cover_id}"
  curl -fsS -X POST "$MEDIA_URL/media/$media_id/complete" -H 'Content-Type: application/json' -d "$body" | jq -r '.data.id'
}

cover_id="$(upload_media "$TMP_DIR/cover.jpg" cover.jpg image image/jpeg)"
video_id="$(upload_media "$TMP_DIR/video.mp4" video.mp4 video video/mp4 "$cover_id")"

post="$(curl -fsS -X POST "$COMMUNITY_URL/admin/masters/community/posts" -H 'Content-Type: application/json' \
  -d "{\"ownerId\":\"$OWNER_ID\",\"masterId\":\"$MASTER_ID\",\"type\":\"video\",\"title\":\"大师广场闭环\",\"content\":\"真实媒体引用\",\"coverMediaId\":$cover_id,\"beliefCode\":\"han_buddhism\",\"assets\":[{\"mediaId\":$video_id,\"assetType\":\"video\",\"sort\":0}],\"submit\":true}")"
post_id="$(jq -r '.data.id' <<<"$post")"
[[ "$(jq -r '.data.status' <<<"$post")" == "pending" ]]

hidden="$(curl -fsS "$COMMUNITY_URL/community/feed")"
[[ "$(jq --arg id "$post_id" '[.data.list[] | select(.id == $id)] | length' <<<"$hidden")" == "0" ]]

approved="$(curl -fsS -X PUT "$COMMUNITY_URL/admin/platform/community/posts/$post_id/approve" -H 'Content-Type: application/json' -d "{\"auditorId\":\"$AUDITOR_ID\"}")"
[[ "$(jq -r '.data.status' <<<"$approved")" == "approved" ]]
visible="$(curl -fsS "$COMMUNITY_URL/community/feed")"
[[ "$(jq --arg id "$post_id" '[.data.list[] | select(.id == $id)] | length' <<<"$visible")" == "1" ]]
[[ -z "$(jq -r --arg id "$post_id" '.data.list[] | select(.id == $id) | .ownerId // empty' <<<"$visible")" ]]

first_like="$(curl -fsS -X POST "$COMMUNITY_URL/community/posts/$post_id/like" -H 'Content-Type: application/json' -d "{\"userId\":\"$USER_ID\"}")"
second_like="$(curl -fsS -X POST "$COMMUNITY_URL/community/posts/$post_id/like" -H 'Content-Type: application/json' -d "{\"userId\":\"$USER_ID\"}")"
[[ "$(jq -r '.data.likeCount' <<<"$first_like")" == "1" ]]
[[ "$(jq -r '.data.likeCount' <<<"$second_like")" == "1" ]]

comment="$(curl -fsS -X POST "$COMMUNITY_URL/community/posts/$post_id/comments" -H 'Content-Type: application/json' -d "{\"userId\":\"$USER_ID\",\"content\":\"评论先审后显\"}")"
comment_id="$(jq -r '.data.id' <<<"$comment")"
[[ "$(jq -r '.data.status' <<<"$comment")" == "pending" ]]
before_review="$(curl -fsS "$COMMUNITY_URL/community/posts/$post_id/comments")"
[[ "$(jq --arg id "$comment_id" '[.data.list[] | select(.id == $id)] | length' <<<"$before_review")" == "0" ]]

curl -fsS -X PUT "$COMMUNITY_URL/admin/platform/community/comments/$comment_id/approve" -H 'Content-Type: application/json' -d "{\"auditorId\":\"$AUDITOR_ID\"}" >/dev/null
after_review="$(curl -fsS "$COMMUNITY_URL/community/posts/$post_id/comments")"
[[ "$(jq --arg id "$comment_id" '[.data.list[] | select(.id == $id)] | length' <<<"$after_review")" == "1" ]]

echo "PASS community closed loop: postId=$post_id commentId=$comment_id"
