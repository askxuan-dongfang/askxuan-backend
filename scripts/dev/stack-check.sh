#!/bin/bash
# Runtime health checks for the local askXuan + OpenIM full backend stack.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

cd "${ROOT}"

fail=0

check_http() {
  local name="$1"
  local url="$2"
  if curl --connect-timeout 3 --max-time 10 -fsS "${url}" >/dev/null 2>&1; then
    echo "OK: ${name} ${url}"
  else
    echo "ERROR: ${name} ${url}"
    fail=1
  fi
}

check_http "askXuan gateway" "http://127.0.0.1:8080/api/v1/health"

compose=(docker compose -f docker-compose.yml -f docker-compose.full.yml)

echo "==> Checking askXuan container health..."
while IFS= read -r service; do
  container_id="$("${compose[@]}" ps -q "${service}")"
  if [ -z "${container_id}" ]; then
    echo "ERROR: ${service} container is not running"
    fail=1
    continue
  fi

  state="$(docker inspect --format '{{.State.Status}}' "${container_id}")"
  health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${container_id}")"
  if [ "${state}" != "running" ] || { [ "${health}" != "healthy" ] && [ "${health}" != "none" ]; }; then
    echo "ERROR: ${service} state=${state} health=${health}"
    fail=1
  fi
done < <("${compose[@]}" config --services)

for rpc_key in temple.rpc master.rpc product.rpc diy.rpc order.rpc payment.rpc; do
  registered=0
  for _ in $(seq 1 10); do
    if docker exec askxuan-etcd etcdctl --endpoints=http://127.0.0.1:2379 \
      get "${rpc_key}" --prefix --keys-only 2>/dev/null | grep -q "${rpc_key}"; then
      registered=1
      break
    fi
    sleep 1
  done
  if [ "${registered}" = "1" ]; then
    echo "OK: ${rpc_key} registered"
  else
    echo "ERROR: ${rpc_key} is not registered in etcd"
    fail=1
  fi
done

openim_secret="${OPENIM_SECRET:-openIM123}"
openim_payload="$(printf '{\"secret\":\"%s\",\"userID\":\"imAdmin\"}' "${openim_secret}")"
openim_response="$(curl --connect-timeout 3 --max-time 10 -fsS -X POST http://127.0.0.1:10002/auth/get_admin_token \
  -H 'Content-Type: application/json' \
  -H "operationID: askxuan-stack-check" \
  -d "${openim_payload}" 2>/dev/null || true)"
if printf '%s' "${openim_response}" | grep -Eq '"errCode"[[:space:]]*:[[:space:]]*0'; then
  echo "OK: OpenIM REST admin token"
else
  echo "ERROR: OpenIM REST admin token（运行 make openim-up 可恢复宿主服务）"
  fail=1
fi

"${compose[@]}" ps

if [ "${fail}" != "0" ]; then
  exit 1
fi

echo "Stack checks OK."
