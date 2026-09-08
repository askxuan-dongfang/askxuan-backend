#!/bin/bash
set -euo pipefail
name=askxuan-commerce-test-$$
repo_dir=$(cd "$(dirname "$0")/.." && pwd)
trap 'docker rm -f "$name" >/dev/null 2>&1 || true' EXIT
docker run -d --rm --name "$name" --tmpfs /var/lib/mysql:rw,size=1g -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=askxuan_order -e MYSQL_INITDB_SKIP_TZINFO=1 -p 127.0.0.1::3306 mysql:8.0 --innodb-flush-log-at-trx-commit=0 --sync-binlog=0 >/dev/null
for ((i=0;i<180;i++));do if docker exec "$name" mysqladmin --protocol=tcp -h127.0.0.1 ping >/dev/null 2>&1;then break;fi;sleep 1;done
port=$(docker port "$name" 3306/tcp);port=${port##*:}
export COMMERCE_TEST_DSN="root@tcp(127.0.0.1:${port})/askxuan_order?charset=utf8mb4&timeout=5s"
cd "$repo_dir/services/commerce/order-service"
go test ./internal/model -run TestMySQLReturnFulfillment -count=1 -v
