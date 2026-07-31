#!/bin/bash
# One-command local Docker startup for middleware + all askXuan Go services.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

cd "${ROOT}"

bash scripts/dev/render-docker-configs.sh

docker compose -f docker-compose.yml -f docker-compose.full.yml up -d --build mysql redis rabbitmq minio etcd

echo "==> Waiting for MySQL..."
for _ in $(seq 1 60); do
  if docker exec askxuan-mysql mysqladmin ping -h 127.0.0.1 -uroot -proot123 >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

echo "==> Checking database schema..."
table_count="$(docker exec askxuan-mysql mysql -N -uroot -proot123 -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='askxuan_temple' AND table_name='temple';" 2>/dev/null || echo 0)"
if [ "${table_count}" = "0" ]; then
  echo "==> First startup detected, initializing database..."
  docker exec -i askxuan-mysql mysql -uroot -proot123 askxuan < db/init.sql
else
  echo "==> Existing database detected, preserving business data."
fi

echo "==> Applying idempotent forward migrations..."
for migration in scripts/db/20*.sql; do
  echo "    $(basename "${migration}")"
  docker exec -i askxuan-mysql mysql -uroot -proot123 < "${migration}"
done

echo "==> Starting all backend services..."
# Recreate every service after middleware/config changes so zrpc leases are
# registered against the current etcd instance instead of a stale endpoint.
docker compose -f docker-compose.yml -f docker-compose.full.yml up -d --build --force-recreate

echo "==> Waiting for gateway..."
for _ in $(seq 1 60); do
  if curl -fsS http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; then
    echo "==> Backend is ready: http://127.0.0.1:8080/api/v1/health"
    exit 0
  fi
  sleep 2
done

echo "ERROR: gateway health check failed. Recent logs:"
docker compose -f docker-compose.yml -f docker-compose.full.yml logs --tail=120 gateway-service
exit 1
