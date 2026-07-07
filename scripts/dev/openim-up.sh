#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OPENIM_VERSION="${OPENIM_VERSION:-v3.8.3}"
OPENIM_DIR="$ROOT_DIR/.local/openim"
OPENIM_ARCHIVE="$OPENIM_DIR/openim-server-$OPENIM_VERSION.tar.gz"
OPENIM_URL="https://github.com/openimsdk/open-im-server/archive/refs/tags/$OPENIM_VERSION.tar.gz"
MAGE_VERSION="${MAGE_VERSION:-v1.15.0}"
OPENIM_KAFKA_IMAGE="${OPENIM_KAFKA_IMAGE:-bitnamilegacy/kafka:3.5.1}"

mkdir -p "$OPENIM_DIR"

find_openim_src() {
  find "$OPENIM_DIR" -maxdepth 1 -type d \
    \( -iname "open-im-server-${OPENIM_VERSION#v}" -o -iname "Open-IM-Server-${OPENIM_VERSION#v}" -o -iname "*open*im*server*" \) \
    | sort | head -1
}

OPENIM_SRC="$(find_openim_src)"
if [ -z "$OPENIM_SRC" ] || [ ! -d "$OPENIM_SRC" ]; then
  echo "==> 下载 OpenIM $OPENIM_VERSION"
  curl -L "$OPENIM_URL" -o "$OPENIM_ARCHIVE"
  tar -xzf "$OPENIM_ARCHIVE" -C "$OPENIM_DIR"
  OPENIM_SRC="$(find_openim_src)"
fi

if [ -z "$OPENIM_SRC" ] || [ ! -d "$OPENIM_SRC" ]; then
  echo "OpenIM 解压目录未找到：$OPENIM_DIR" >&2
  exit 1
fi

if [ "${OPENIM_MINIMAL:-0}" = "1" ] && [ -f "$OPENIM_SRC/start-config.yml" ]; then
  echo "==> 使用本地验收最小实例配置（api/auth/user/msggateway/msg/msgtransfer/conversation=1，其余=0）"
  perl -0pi -e '
    s/openim-api:\s*\d+/openim-api: 1/;
    s/openim-rpc-user:\s*\d+/openim-rpc-user: 1/;
    s/openim-msggateway:\s*\d+/openim-msggateway: 1/;
    s/openim-rpc-auth:\s*\d+/openim-rpc-auth: 1/;
    s/openim-rpc-msg:\s*\d+/openim-rpc-msg: 1/;
    s/openim-msgtransfer:\s*\d+/openim-msgtransfer: 1/;
    s/openim-rpc-conversation:\s*\d+/openim-rpc-conversation: 1/;
    s/openim-crontask:\s*\d+/openim-crontask: 0/;
    s/openim-push:\s*\d+/openim-push: 0/;
    s/openim-rpc-group:\s*\d+/openim-rpc-group: 0/;
    s/openim-rpc-friend:\s*\d+/openim-rpc-friend: 0/;
    s/openim-rpc-third:\s*\d+/openim-rpc-third: 0/;
  ' "$OPENIM_SRC/start-config.yml"
fi
if [ -f "$OPENIM_SRC/config/mongodb.yml" ]; then
  perl -0pi -e 's/address:\s*\[\s*localhost:37017\s*\]/address: [ 127.0.0.1:37017 ]/' "$OPENIM_SRC/config/mongodb.yml"
fi

COMPOSE_FILE="$(find "$OPENIM_SRC" -type f \( -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name 'compose*.yaml' \) | head -1)"
if [ -z "$COMPOSE_FILE" ]; then
  echo "未找到 OpenIM docker compose 文件：$OPENIM_SRC" >&2
  exit 1
fi

echo "==> 启动 OpenIM compose: $COMPOSE_FILE"
KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" docker compose -f "$COMPOSE_FILE" up -d

if ! docker exec mongo sh -lc 'pgrep mongod >/dev/null 2>&1' >/dev/null 2>&1; then
  echo "==> OpenIM Mongo 容器存在但 mongod 未运行，备份本地 Mongo 数据后重建"
  (
    cd "$OPENIM_SRC"
    ts="$(date +%Y%m%d-%H%M%S)"
    if [ -d components/mongodb ]; then
      mv components/mongodb "components/mongodb.bak-$ts"
    fi
    KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" docker compose -f "$COMPOSE_FILE" down
    KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" docker compose -f "$COMPOSE_FILE" up -d
  )
fi

echo "==> 等待 OpenIM Mongo 用户就绪"
for i in $(seq 1 90); do
  if timeout 8 docker exec mongo mongosh -u root -p openIM123 --authenticationDatabase admin \
    --eval 'db.getSiblingDB("openim_v3").getUser("openIM")' >/dev/null 2>&1; then
    break
  fi
  if [ "$i" = "90" ]; then
    echo "OpenIM Mongo 未就绪，请检查 docker logs mongo" >&2
    exit 1
  fi
  sleep 2
done

echo "==> 构建并启动 OpenIM 服务本体"
(
  cd "$OPENIM_SRC"
  GOWORK=off go run "github.com/magefile/mage@$MAGE_VERSION" build
  GOWORK=off go run "github.com/magefile/mage@$MAGE_VERSION" start
)

echo "==> 等待 OpenIM REST API http://127.0.0.1:10002"
for i in $(seq 1 90); do
  if curl -fsS -X POST http://127.0.0.1:10002/auth/get_admin_token \
    -H 'Content-Type: application/json' \
    -H "operationID: askxuan-openim-ready-$i" \
    -d '{"secret":"openIM123","userID":"imAdmin"}' >/dev/null 2>&1; then
    echo "==> OpenIM 已就绪"
    exit 0
  fi
  sleep 2
done

echo "OpenIM REST API 未就绪，请检查 docker compose 日志。" >&2
KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" docker compose -f "$COMPOSE_FILE" ps || true
exit 1
