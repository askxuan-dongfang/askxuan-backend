#!/usr/bin/env bash
# Dedicated ephemeral server. Never point these fixtures at an application DB.
set -euo pipefail
cd "$(dirname "$0")/.."
name=askxuan-wish-test-20261014
if docker inspect "$name" >/dev/null 2>&1; then
  echo "Test container already exists; finish its owning run first." >&2; exit 1
fi
docker run --name "$name" --rm -d --label codex.task=wish-completeness-20261014 -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -p 127.0.0.1:13316:3306 --tmpfs /var/lib/mysql mysql:8.0 >/dev/null
trap 'docker stop "$name" >/dev/null' EXIT
for attempt in $(seq 1 60); do
  if docker exec "$name" mysqladmin -h127.0.0.1 ping --silent >/dev/null 2>&1; then break; fi
  sleep 1
done
export ASKXUAN_WISH_TEST_DSN='root@tcp(127.0.0.1:13316)/askxuan_product?charset=utf8mb4'
go test -count=1 -v ./services/commerce/product-service/internal/model -run TestIntentionMySQL
go test -count=1 -v ./services/content/master-service/internal/model -run TestMasterMediaAndIntentMySQL

python3 scripts/test-wish-migrations.py
go test -count=1 -v ./services/content/temple-service/internal/model -run TestTempleWishMediaMySQL
