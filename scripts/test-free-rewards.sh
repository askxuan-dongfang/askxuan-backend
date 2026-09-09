#!/usr/bin/env bash
set -euo pipefail
REWARDS_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REWARDS_CONTAINER="askxuan-rewards-test-$$"
trap 'docker rm -f "$REWARDS_CONTAINER" >/dev/null 2>&1 || true' EXIT
docker run -d --rm --name "$REWARDS_CONTAINER" --tmpfs /var/lib/mysql:rw,size=1g -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=askxuan_rewards_test -e MYSQL_INITDB_SKIP_TZINFO=1 -p 127.0.0.1::3306 mysql:8.0 --innodb-flush-log-at-trx-commit=0 --sync-binlog=0 >/dev/null
ready=0
for ((i=0;i<120;i++)); do if docker exec "$REWARDS_CONTAINER" mysqladmin --protocol=tcp -h127.0.0.1 ping >/dev/null 2>&1; then ready=1;break;fi;sleep 1;done
[[ "$ready" == 1 ]]
REWARDS_PORT="$(docker port "$REWARDS_CONTAINER" 3306/tcp)"
export REWARDS_TEST_DSN="root@tcp(127.0.0.1:${REWARDS_PORT##*:})/askxuan_rewards_test?charset=utf8mb4&timeout=5s"
export GOCACHE="${GOCACHE:-/private/tmp/askxuan-rewards-go-cache}"
cd "$REWARDS_ROOT/services/operation/marketing-service"
go test ./... -count=1 -v
