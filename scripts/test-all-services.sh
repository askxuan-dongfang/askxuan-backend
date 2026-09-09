#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOCACHE="${GOCACHE:-/private/tmp/askxuan-rewards-go-cache}"
for module in common services/*/*-service; do
 echo "TEST $module"
 (cd "$module" && go test ./...)
done
