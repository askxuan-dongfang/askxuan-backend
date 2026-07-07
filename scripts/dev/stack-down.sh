#!/bin/bash
# Stop the complete local backend stack. Data volumes/directories are preserved.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

cd "${ROOT}"

echo "==> Stop askXuan stack"
docker compose -f docker-compose.yml -f docker-compose.full.yml down

echo "==> Stop OpenIM stack"
bash scripts/dev/openim-down.sh

echo "==> Full backend stack stopped"
