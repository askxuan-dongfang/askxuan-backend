#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080/api/v1}"
response="$(curl -fsS "$BASE_URL/intentions?code=peace&size=100")"

[[ "$(jq -r '.code' <<<"$response")" == "0" ]]
[[ "$(jq '[.data.tags[] | select(.code == "peace")] | length' <<<"$response")" == "1" ]]
[[ "$(jq '[.data.list[] | select(.resourceType == "service")] | length' <<<"$response")" -gt 0 ]]
[[ "$(jq '[.data.list[] | select(.resourceType == "master")] | length' <<<"$response")" -gt 0 ]]
[[ "$(jq '[.data.list[] | select(.resourceType != "service" and .resourceType != "master")] | length' <<<"$response")" == "0" ]]
[[ "$(jq '[.data.list[] | select((.sourceId | length) == 0 or .price < 0 or (.orderTarget | length) == 0)] | length' <<<"$response")" == "0" ]]
[[ "$(jq '[.data.list[] | select(.resourceType == "service" and ((.orderTarget | startswith("service:") | not) or (.templeCode | length) == 0 or (.serviceCode | length) == 0))] | length' <<<"$response")" == "0" ]]
[[ "$(jq '[.data.list[] | select(.resourceType == "master" and ((.orderTarget | startswith("master:") | not) or (.masterCode | length) == 0 or (.serviceCode | length) == 0))] | length' <<<"$response")" == "0" ]]

unknown="$(curl -sS "$BASE_URL/intentions?code=unknown")"
[[ "$(jq -r '.code' <<<"$unknown")" != "0" ]]

echo "PASS intention closed loop: resources=$(jq -r '.data.total' <<<"$response") types=service,master"
