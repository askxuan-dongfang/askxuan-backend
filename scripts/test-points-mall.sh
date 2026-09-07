#!/usr/bin/env bash
# Isolated MySQL points tests. Never connects to an existing business database.
set -euo pipefail
POINTS_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
POINTS_CONTAINER="askxuan-points-test-$$"
cleanup() { docker rm -f "$POINTS_CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run -d --rm --name "$POINTS_CONTAINER" \
  --tmpfs /var/lib/mysql:rw,size=1g \
  -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=askxuan_payment \
  -e MYSQL_INITDB_SKIP_TZINFO=1 -p 127.0.0.1::3306 \
  mysql:8.0 --innodb-flush-log-at-trx-commit=0 --sync-binlog=0 >/dev/null
ready=0
for ((i=0; i<180; i++)); do
  if docker exec "$POINTS_CONTAINER" mysqladmin --protocol=tcp -h127.0.0.1 ping >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
if [[ "$ready" != 1 ]]; then docker logs --tail 20 "$POINTS_CONTAINER"; exit 1; fi
POINTS_PORT="$(docker port "$POINTS_CONTAINER" 3306/tcp)"
POINTS_PORT="${POINTS_PORT##*:}"
export POINTS_TEST_DSN="root@tcp(127.0.0.1:${POINTS_PORT})/askxuan_payment?charset=utf8mb4&timeout=5s"
cd "$POINTS_ROOT/services/commerce/payment-service"
go test ./internal/points -count=1
go test ./internal/model -run TestMySQLPaymentAndRefundPoints -count=1
go test ./internal/handler -count=1
