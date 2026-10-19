#!/usr/bin/env bash
# Disposable local-only transactional regression. Never accepts an external DSN.
set -euo pipefail
cd "$(dirname "$0")/.."
container="askxuan-growth-test-$$"
cleanup() { docker rm -f "$container" >/dev/null 2>&1 || true; }
trap cleanup EXIT
# Random localhost port avoids developer databases and other CI jobs.
docker run -d --name "$container" -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=askxuan -p 127.0.0.1::3306 mysql:8.0 >/dev/null
ready=0
for attempt in $(seq 1 60); do
 # The image starts a temporary socket-only server while initializing schemas.
 # Wait for the final TCP server and requested database, not merely a ping.
 if docker exec "$container" mysql -h127.0.0.1 -uroot -D askxuan -e 'SELECT 1' >/dev/null 2>&1; then ready=1; break; fi
 sleep 1
done
[[ "$ready" == 1 ]] || { docker logs "$container"; exit 1; }
docker exec -i "$container" mysql -uroot askxuan < db/init.sql
# init.sql includes growth DDL. Load the existing cash wallet additive DDL only.
sed '/^GRANT /,$d' scripts/db/20260926_balance_wallet.sql | docker exec -i "$container" mysql -uroot
# Creator rewards predate growth but their additive tables are not in init.sql.
sed -n '/^CREATE TABLE IF NOT EXISTS askxuan_diy.creator_reward_policy/,$p' scripts/db/20261013_wishes_creator_rewards.sql | sed '/^GRANT /d' | docker exec -i "$container" mysql -uroot
port=$(docker port "$container" 3306/tcp | sed 's/.*://')
export GROWTH_TEST_ISOLATED=1
export GROWTH_TEST_DSN="root@tcp(127.0.0.1:$port)/askxuan_payment?charset=utf8mb4&parseTime=false"
go test ./services/commerce/payment-service/internal/growth -count=1 -v
go test ./services/commerce/diy-service/internal/model -run TestCreatorNetGrowthMySQL -count=1 -v
