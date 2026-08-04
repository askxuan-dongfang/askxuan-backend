#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OPENIM_VERSION="${OPENIM_VERSION:-v3.8.3}"
OPENIM_DIR="$ROOT_DIR/.local/openim"
OPENIM_ARCHIVE="$OPENIM_DIR/openim-server-$OPENIM_VERSION.tar.gz"
OPENIM_URL="https://github.com/openimsdk/open-im-server/archive/refs/tags/$OPENIM_VERSION.tar.gz"
MAGE_VERSION="${MAGE_VERSION:-v1.15.0}"
OPENIM_KAFKA_IMAGE="${OPENIM_KAFKA_IMAGE:-docker.1ms.run/bitnamilegacy/kafka:3.5.1}"
OPENIM_LAUNCH_LABEL="${OPENIM_LAUNCH_LABEL:-com.askxuan.openim.start}"
OPENIM_SECRET="${OPENIM_SECRET:-openIM123}"
OPENIM_MONGO_IMAGE="${OPENIM_MONGO_IMAGE:-docker.1ms.run/library/mongo:7.0}"
OPENIM_REDIS_IMAGE="${OPENIM_REDIS_IMAGE:-redis:7.0}"
OPENIM_ETCD_IMAGE="${OPENIM_ETCD_IMAGE:-quay.io/coreos/etcd:v3.5.15}"
OPENIM_MINIO_IMAGE="${OPENIM_MINIO_IMAGE:-minio/minio:latest}"
OPENIM_WEB_FRONT_IMAGE="${OPENIM_WEB_FRONT_IMAGE:-registry.cn-hangzhou.aliyuncs.com/openimsdk/openim-web-front:release-v3.8.3}"
OPENIM_ADMIN_FRONT_IMAGE="${OPENIM_ADMIN_FRONT_IMAGE:-registry.cn-hangzhou.aliyuncs.com/openimsdk/openim-admin-front:release-v1.8.4}"

openim_ready() {
  local response
  response="$(curl -fsS --max-time 5 -X POST http://127.0.0.1:10002/auth/get_admin_token \
    -H 'Content-Type: application/json' \
    -H "operationID: askxuan-openim-ready" \
    -d "{\"secret\":\"$OPENIM_SECRET\",\"userID\":\"imAdmin\"}" 2>/dev/null)" || return 1
  printf '%s' "$response" | grep -Eq '"errCode"[[:space:]]*:[[:space:]]*0'
}

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

CONFIG_CHANGED=0
if [ -f "$ROOT_DIR/configs/openim/webhooks.yml" ] && ! cmp -s "$ROOT_DIR/configs/openim/webhooks.yml" "$OPENIM_SRC/config/webhooks.yml"; then
  echo "==> 应用问玄付费预约聊天 webhook 配置"
  cp "$ROOT_DIR/configs/openim/webhooks.yml" "$OPENIM_SRC/config/webhooks.yml"
  CONFIG_CHANGED=1
fi

if [ -f "$OPENIM_SRC/start-config.yml" ]; then
  CONFIG_BEFORE="$(cksum "$OPENIM_SRC/start-config.yml")"
  if [ "${OPENIM_MINIMAL:-0}" = "1" ]; then
    echo "==> 使用本地验收最小实例配置（保留单聊和历史消息所需服务）"
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
      s/openim-rpc-friend:\s*\d+/openim-rpc-friend: 1/;
      s/openim-rpc-third:\s*\d+/openim-rpc-third: 0/;
    ' "$OPENIM_SRC/start-config.yml"
  else
    echo "==> 使用完整 OpenIM 服务配置"
    perl -0pi -e '
      s/(openim-(?:api|crontask|msggateway|push|msgtransfer|rpc-(?:user|conversation|auth|group|friend|msg|third))):\s*\d+/$1: 1/g;
    ' "$OPENIM_SRC/start-config.yml"
  fi
  if [ "$CONFIG_BEFORE" != "$(cksum "$OPENIM_SRC/start-config.yml")" ]; then
    CONFIG_CHANGED=1
  fi
fi
if [ -f "$OPENIM_SRC/config/mongodb.yml" ]; then
  perl -0pi -e 's/address:\s*\[\s*localhost:37017\s*\]/address: [ 127.0.0.1:37017 ]/' "$OPENIM_SRC/config/mongodb.yml"
fi
if [ -f "$OPENIM_SRC/config/share.yml" ]; then
  CONFIG_BEFORE="$(cksum "$OPENIM_SRC/config/share.yml")"
  OPENIM_SECRET="$OPENIM_SECRET" perl -0pi -e 's/^secret:\s*.*$/secret: $ENV{OPENIM_SECRET}/m' "$OPENIM_SRC/config/share.yml"
  if [ "$CONFIG_BEFORE" != "$(cksum "$OPENIM_SRC/config/share.yml")" ]; then
    CONFIG_CHANGED=1
  fi
fi

# OpenIM stays private until a TLS reverse proxy is configured. All OpenIM
# processes run on this host, so loopback listeners preserve service discovery.
CONFIG_BEFORE="$(find "$OPENIM_SRC/config" -maxdepth 1 -type f -name 'openim-*.yml' -exec cksum {} + | cksum)"
find "$OPENIM_SRC/config" -maxdepth 1 -type f -name 'openim-*.yml' -exec \
  perl -0pi -e 's/listenIP:\s*0\.0\.0\.0/listenIP: 127.0.0.1/g' {} +
if [ "$CONFIG_BEFORE" != "$(find "$OPENIM_SRC/config" -maxdepth 1 -type f -name 'openim-*.yml' -exec cksum {} + | cksum)" ]; then
  CONFIG_CHANGED=1
fi

COMPOSE_FILE="$(find "$OPENIM_SRC" -type f \( -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name 'compose*.yaml' \) | head -1)"
if [ -z "$COMPOSE_FILE" ]; then
  echo "未找到 OpenIM docker compose 文件：$OPENIM_SRC" >&2
  exit 1
fi

perl -0pi -e 's/"(37017|16379|12379|12380|19094|10005|19090|11001|11002):/"127.0.0.1:$1:/g' "$COMPOSE_FILE"

echo "==> 启动 OpenIM compose: $COMPOSE_FILE"
MONGO_IMAGE="$OPENIM_MONGO_IMAGE" \
REDIS_IMAGE="$OPENIM_REDIS_IMAGE" \
ETCD_IMAGE="$OPENIM_ETCD_IMAGE" \
KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" \
MINIO_IMAGE="$OPENIM_MINIO_IMAGE" \
OPENIM_WEB_FRONT_IMAGE="$OPENIM_WEB_FRONT_IMAGE" \
OPENIM_ADMIN_FRONT_IMAGE="$OPENIM_ADMIN_FRONT_IMAGE" \
  docker compose -f "$COMPOSE_FILE" up -d

if ! docker exec mongo sh -lc 'pgrep mongod >/dev/null 2>&1' >/dev/null 2>&1; then
  echo "==> OpenIM Mongo 容器存在但 mongod 未运行，备份本地 Mongo 数据后重建"
  (
    cd "$OPENIM_SRC"
    ts="$(date +%Y%m%d-%H%M%S)"
    if [ -d components/mongodb ]; then
      mv components/mongodb "components/mongodb.bak-$ts"
    fi
    MONGO_IMAGE="$OPENIM_MONGO_IMAGE" REDIS_IMAGE="$OPENIM_REDIS_IMAGE" \
      ETCD_IMAGE="$OPENIM_ETCD_IMAGE" KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" \
      MINIO_IMAGE="$OPENIM_MINIO_IMAGE" OPENIM_WEB_FRONT_IMAGE="$OPENIM_WEB_FRONT_IMAGE" \
      OPENIM_ADMIN_FRONT_IMAGE="$OPENIM_ADMIN_FRONT_IMAGE" docker compose -f "$COMPOSE_FILE" down
    MONGO_IMAGE="$OPENIM_MONGO_IMAGE" REDIS_IMAGE="$OPENIM_REDIS_IMAGE" \
      ETCD_IMAGE="$OPENIM_ETCD_IMAGE" KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" \
      MINIO_IMAGE="$OPENIM_MINIO_IMAGE" OPENIM_WEB_FRONT_IMAGE="$OPENIM_WEB_FRONT_IMAGE" \
      OPENIM_ADMIN_FRONT_IMAGE="$OPENIM_ADMIN_FRONT_IMAGE" docker compose -f "$COMPOSE_FILE" up -d
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

if [ "$CONFIG_CHANGED" = "0" ] && openim_ready; then
  echo "==> OpenIM 服务本体已就绪，跳过重启"
  exit 0
fi

GOOS="$(go env GOOS)"
GOARCH="$(go env GOARCH)"
OPENIM_API_BIN="$OPENIM_SRC/_output/bin/platforms/$GOOS/$GOARCH/openim-api"
NEEDS_BUILD=0
if [ "${OPENIM_FORCE_REBUILD:-0}" = "1" ] || [ ! -x "$OPENIM_API_BIN" ]; then
  NEEDS_BUILD=1
elif find "$OPENIM_SRC/cmd" "$OPENIM_SRC/internal" "$OPENIM_SRC/pkg" \
  -type f -name '*.go' -newer "$OPENIM_API_BIN" -print -quit 2>/dev/null | grep -q .; then
  NEEDS_BUILD=1
elif [ "$OPENIM_SRC/go.mod" -nt "$OPENIM_API_BIN" ] || [ "$OPENIM_SRC/go.sum" -nt "$OPENIM_API_BIN" ]; then
  NEEDS_BUILD=1
fi

echo "==> 启动 OpenIM 服务本体"
OPENIM_LAUNCH_LOG="$OPENIM_SRC/_output/logs/askxuan-launcher.log"
mkdir -p "$(dirname "$OPENIM_LAUNCH_LOG")"
if [ "$NEEDS_BUILD" = "1" ]; then
  echo "==> OpenIM 二进制缺失或源码已变化，开始构建"
  (
    cd "$OPENIM_SRC"
    GOWORK=off go run "github.com/magefile/mage@$MAGE_VERSION" build
  )
else
  echo "==> 复用现有 OpenIM 二进制"
fi

if [ "$(uname -s)" = "Darwin" ]; then
  launchctl remove "$OPENIM_LAUNCH_LABEL" >/dev/null 2>&1 || true
  LAUNCH_CMD="$(printf 'cd %q && env GOWORK=off go run %q start && while :; do sleep 3600; done' \
    "$OPENIM_SRC" "github.com/magefile/mage@$MAGE_VERSION")"
  launchctl submit -l "$OPENIM_LAUNCH_LABEL" \
    -o "$OPENIM_LAUNCH_LOG" -e "$OPENIM_LAUNCH_LOG" \
    -- /bin/zsh -lc "$LAUNCH_CMD"
else
  (
    cd "$OPENIM_SRC"
    if ! GOWORK=off nohup go run "github.com/magefile/mage@$MAGE_VERSION" start \
      </dev/null >>"$OPENIM_LAUNCH_LOG" 2>&1; then
      echo "OpenIM 服务启动失败，最近日志如下：" >&2
      tail -n 80 "$OPENIM_LAUNCH_LOG" >&2 || true
      exit 1
    fi
  )
fi

echo "==> 等待 OpenIM REST API http://127.0.0.1:10002"
for i in $(seq 1 90); do
  if openim_ready; then
    echo "==> OpenIM 已就绪"
    exit 0
  fi
  sleep 2
done

echo "OpenIM REST API 未就绪，请检查 docker compose 日志。" >&2
KAFKA_IMAGE="$OPENIM_KAFKA_IMAGE" docker compose -f "$COMPOSE_FILE" ps || true
exit 1
