#!/bin/bash
# Start the complete local backend stack: OpenIM + askXuan middleware + askXuan Go services.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

cd "${ROOT}"

echo "==> Stop old host askXuan processes if any"
make -s stop-all || true

echo "==> Preflight checks"
bash scripts/dev/stack-preflight.sh

echo "==> Start OpenIM stack"
OPENIM_MINIMAL="${OPENIM_MINIMAL:-0}" bash scripts/dev/openim-up.sh

echo "==> Start askXuan stack"
bash scripts/dev/docker-up-all.sh

echo "==> Full backend stack is ready"
echo "    askXuan gateway: http://127.0.0.1:8080/api/v1/health"
echo "    OpenIM REST:     http://127.0.0.1:10002"
echo "    OpenIM Web:      http://127.0.0.1:11001"
echo "    OpenIM Admin:    http://127.0.0.1:11002"
