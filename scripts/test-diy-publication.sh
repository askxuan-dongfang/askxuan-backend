#!/usr/bin/env bash
# Disposable database only. No external DSN/config or production credentials.
set -euo pipefail
cd "$(dirname "$0")/.."
container="askxuan-diy-publication-test-$$"
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
sed -n '/^CREATE TABLE IF NOT EXISTS askxuan_diy.creator_reward_policy/,$p' scripts/db/20261013_wishes_creator_rewards.sql | sed '/^GRANT /d' | docker exec -i "$container" mysql -uroot
# The repeat proves that the exact production migration is re-runnable.
docker exec -i "$container" mysql -uroot < scripts/db/20261020_diy_publication_gate.sql
docker exec -i "$container" mysql -uroot < scripts/db/20261020_diy_publication_gate.sql
port=$(docker port "$container" 3306/tcp | sed 's/.*://')
export DIY_PUBLICATION_TEST_ISOLATED=1
export DIY_PUBLICATION_TEST_DSN="root@tcp(127.0.0.1:$port)/askxuan_diy?charset=utf8mb4&parseTime=false"
export DIY_STUDIO_TEST=1
GOMAXPROCS=2 go test -p 2 ./services/commerce/diy-service/internal/model -run 'TestPublicationMySQL' -count=1 -v
GOMAXPROCS=2 go test -p 2 ./services/commerce/diy-service/internal/handler -run TestStudioIntegration -count=1 -v
