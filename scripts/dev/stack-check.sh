#!/bin/bash
# Runtime health checks for the local askXuan + OpenIM full backend stack.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

cd "${ROOT}"

fail=0

check_http() {
  local name="$1"
  local url="$2"
  if curl -fsS "${url}" >/dev/null 2>&1; then
    echo "OK: ${name} ${url}"
  else
    echo "ERROR: ${name} ${url}"
    fail=1
  fi
}

check_http "askXuan gateway" "http://127.0.0.1:8080/api/v1/health"

if curl -fsS -X POST http://127.0.0.1:10002/auth/get_admin_token \
  -H 'Content-Type: application/json' \
  -H "operationID: askxuan-stack-check" \
  -d '{"secret":"openIM123","userID":"imAdmin"}' >/dev/null 2>&1; then
  echo "OK: OpenIM REST admin token"
else
  echo "ERROR: OpenIM REST admin token"
  fail=1
fi

docker compose -f docker-compose.yml -f docker-compose.full.yml ps

if [ "${fail}" != "0" ]; then
  exit 1
fi

echo "Stack checks OK."
