#!/bin/bash
# Preflight checks for the local askXuan + OpenIM full backend stack.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OPENIM_VERSION="${OPENIM_VERSION:-v3.8.3}"
OPENIM_DIR="${ROOT}/.local/openim"

find_openim_src() {
  find "${OPENIM_DIR}" -maxdepth 1 -type d \
    \( -iname "open-im-server-${OPENIM_VERSION#v}" -o -iname "Open-IM-Server-${OPENIM_VERSION#v}" -o -iname "*open*im*server*" \) \
    | sort | head -1
}

fail=0

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: missing command: $1"
    fail=1
  fi
}

docker_names_for_port() {
  local port="$1"
  docker ps --format '{{.Names}}	{{.Ports}}' 2>/dev/null \
    | awk -v p="${port}" '$0 ~ ":" p "->" || $0 ~ ":" p "-" || $0 ~ "-" p "->" { print $1 }' \
    | tr '\n' ' '
}

check_port() {
  local port="$1"
  local expected="$2"
  local label="$3"
  local names
  local listeners

  listeners="$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -z "${listeners}" ]; then
    return 0
  fi

  names="$(docker_names_for_port "${port}")"
  if [ -n "${names}" ] && printf '%s\n' "${names}" | grep -Eq "${expected}"; then
    echo "OK: port ${port} already held by expected container(s): ${names}"
    return 0
  fi

  if echo "${listeners}" | awk 'NR > 1 { print $1 }' | grep -Eq "${expected}"; then
    echo "OK: port ${port} already held by expected host process (${label})"
    return 0
  fi

  echo "ERROR: port ${port} is occupied by an unexpected process/container (${label})"
  echo "${listeners}" | sed 's/^/  /'
  [ -n "${names}" ] && echo "  docker containers: ${names}"
  fail=1
}

need_cmd docker
need_cmd lsof
need_cmd curl
need_cmd go

if ! docker info >/dev/null 2>&1; then
  echo "ERROR: Docker is not running or current user cannot access Docker."
  fail=1
fi

OPENIM_SRC="$(find_openim_src || true)"
if [ -z "${OPENIM_SRC}" ]; then
  echo "WARN: OpenIM source directory not found yet; scripts/dev/openim-up.sh will download it."
elif [ ! -f "${OPENIM_SRC}/docker-compose.yml" ]; then
  echo "ERROR: OpenIM compose file missing under ${OPENIM_SRC}"
  fail=1
fi

cd "${ROOT}"
docker compose -f docker-compose.yml -f docker-compose.full.yml config --quiet

# askXuan middleware + services
check_port 3306 'askxuan-mysql' 'askXuan MySQL'
check_port 6379 'askxuan-redis' 'askXuan Redis'
check_port 5672 'askxuan-rabbitmq' 'askXuan RabbitMQ AMQP'
check_port 15672 'askxuan-rabbitmq' 'askXuan RabbitMQ Console'
check_port 9000 'askxuan-minio' 'askXuan MinIO API'
check_port 9001 'askxuan-minio' 'askXuan MinIO Console'
check_port 2379 'askxuan-etcd' 'askXuan etcd client'
check_port 2380 'askxuan-etcd' 'askXuan etcd peer'
for port in 8080 8081 8082 8083 8084 8085 8086 8088 8089 8090 8091 8092 8093 8094 8095 8096 8097 8098 9088; do
  check_port "${port}" 'askxuan-.*-service|gateway|auth-serv|user|temple|master|booking|product|diy|order|payment|finance|review|audit|message|logistics|marketing|file|ai' "askXuan backend service ${port}"
done

# OpenIM docker compose + host services. These are intentionally separate from askXuan.
check_port 37017 'mongo|openim.*mongo' 'OpenIM MongoDB'
check_port 16379 'redis|openim.*redis' 'OpenIM Redis'
check_port 12379 'etcd|openim.*etcd' 'OpenIM etcd client'
check_port 12380 'etcd|openim.*etcd' 'OpenIM etcd peer'
check_port 19094 'kafka|openim.*kafka' 'OpenIM Kafka external'
check_port 10005 'minio|openim.*minio' 'OpenIM MinIO API'
check_port 19090 'minio|openim.*minio' 'OpenIM MinIO Console'
check_port 11001 'openim-web-front' 'OpenIM Web Front'
check_port 11002 'openim-admin-front' 'OpenIM Admin Front'
check_port 10001 'openim|open-im|mage' 'OpenIM WebSocket'
check_port 10002 'openim|open-im|mage' 'OpenIM REST API'

if [ "${fail}" != "0" ]; then
  echo "Preflight failed."
  exit 1
fi

echo "Preflight OK."
