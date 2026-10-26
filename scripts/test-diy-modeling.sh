#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
container="askxuan-diy-modeling-test-$$"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT
docker run -d --name "$container" -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=askxuan -p 127.0.0.1::3306 mysql:8.0 >/dev/null
ready=0
for attempt in $(seq 1 60); do
 if docker exec "$container" mysql -h127.0.0.1 -uroot -D askxuan -e 'SELECT 1' >/dev/null 2>&1; then ready=1; break; fi
 sleep 1
done
[[ "$ready" == 1 ]] || { docker logs "$container"; exit 1; }
docker exec -i "$container" mysql -uroot askxuan < db/init.sql
docker exec -i "$container" mysql -uroot < scripts/db/20260911_diy_studio.sql
docker exec -i "$container" mysql -uroot < scripts/db/20261026_diy_material_modeling.sql
docker exec -i "$container" mysql -uroot < scripts/db/20261026_diy_material_modeling.sql
port=$(docker port "$container" 3306/tcp | sed 's/.*://')
export DIY_MODELING_TEST_ISOLATED=1
export DIY_MODELING_TEST_DSN="root@tcp(127.0.0.1:$port)/askxuan_diy?charset=utf8mb4&parseTime=false"
GOMAXPROCS=2 go test -p 2 ./services/commerce/diy-service/internal/modeling -run TestModelingMySQL -count=1 -v
