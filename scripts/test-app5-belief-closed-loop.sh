#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080/api/v1}"
CODES=(han_buddhism tibetan_buddhism daoism folk)

for code in "${CODES[@]}"; do
  profile="$(curl -fsS "$BASE_URL/beliefs/$code")"
  [[ "$(jq -r '.code' <<<"$profile")" == "0" ]]
  [[ "$(jq -r '.data.code' <<<"$profile")" == "$code" ]]
  [[ -n "$(jq -r '.data.name' <<<"$profile")" ]]
  [[ -n "$(jq -r '.data.description' <<<"$profile")" ]]
done

assert_filter() {
  local resource="$1" code="$2" response
  response="$(curl -fsS "$BASE_URL/$resource?beliefCode=$code&size=100")"
  [[ "$(jq -r '.code' <<<"$response")" == "0" ]]
  [[ "$(jq -r '.data.total' <<<"$response")" -gt 0 ]]
  [[ "$(jq --arg code "$code" '[.data.list[] | select(.beliefCode != $code)] | length' <<<"$response")" == "0" ]]
}

assert_filter temples han_buddhism
assert_filter temples tibetan_buddhism
assert_filter temples daoism
assert_filter masters han_buddhism
assert_filter masters tibetan_buddhism
assert_filter masters daoism

admin_login="$(curl -fsS -X POST "$BASE_URL/auth/admin/login" -H 'Content-Type: application/json' \
  -d '{"account":"admin","password":"123456"}')"
admin_token="$(jq -r '.data.accessToken' <<<"$admin_login")"
profile="$(curl -fsS "$BASE_URL/beliefs/han_buddhism")"
update_body="$(jq -c '.data | {name,summary,description,coverImage,sort}' <<<"$profile")"
updated="$(curl -fsS -X PUT "$BASE_URL/admin/platform/beliefs/han_buddhism" \
  -H "Authorization: Bearer $admin_token" -H 'Content-Type: application/json' -d "$update_body")"
[[ "$(jq -r '.data.code' <<<"$updated")" == "han_buddhism" ]]

echo "PASS belief closed loop: profiles=${#CODES[@]} filters=6 adminUpdate=ok"
