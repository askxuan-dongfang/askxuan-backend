#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OPENIM_VERSION="${OPENIM_VERSION:-v3.8.3}"
OPENIM_DIR="$ROOT_DIR/.local/openim"
MAGE_VERSION="${MAGE_VERSION:-v1.15.0}"
OPENIM_KAFKA_IMAGE="${OPENIM_KAFKA_IMAGE:-bitnamilegacy/kafka:3.5.1}"
OPENIM_LAUNCH_LABEL="${OPENIM_LAUNCH_LABEL:-com.askxuan.openim.start}"
OPENIM_SRC="$(find "$OPENIM_DIR" -maxdepth 1 -type d \( -iname "open-im-server-${OPENIM_VERSION#v}" -o -iname "Open-IM-Server-${OPENIM_VERSION#v}" -o -iname "*open*im*server*" \) | sort | head -1 || true)"

if [ -z "$OPENIM_SRC" ]; then
  echo "未找到 OpenIM 目录，跳过。"
  exit 0
fi

COMPOSE_FILE="$(find "$OPENIM_SRC" -type f \( -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name 'compose*.yaml' \) | head -1 || true)"
if [ -z "$COMPOSE_FILE" ]; then
  echo "未找到 OpenIM compose 文件，跳过。"
  exit 0
fi

echo "==> 停止 OpenIM compose: $COMPOSE_FILE"
if [ "$(uname -s)" = "Darwin" ]; then
  launchctl remove "$OPENIM_LAUNCH_LABEL" >/dev/null 2>&1 || true
fi
(
  cd "$OPENIM_SRC"
  GOWORK=off go run "github.com/magefile/mage@$MAGE_VERSION" stop || true
)
KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" docker compose -f "$COMPOSE_FILE" down
