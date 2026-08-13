#!/usr/bin/env bash
# Synchronize ECS RabbitMQ credentials into the broker and mounted service configs.

set -euo pipefail

SECRETS_FILE="${SECRETS_FILE:-/opt/askxuan/runtime/secrets.env}"
CONFIG_ROOT="${CONFIG_ROOT:-/opt/askxuan/backend/.docker/etc}"
RABBIT_CONTAINER="${RABBIT_CONTAINER:-askxuan-rabbitmq}"

read_secret() {
  sed -n "s/^$1=//p" "$SECRETS_FILE" | tail -1
}

rabbit_user="$(read_secret RABBITMQ_USER)"
rabbit_password="$(read_secret RABBITMQ_PASSWORD)"
if [ -z "$rabbit_user" ] || [ -z "$rabbit_password" ]; then
  echo "RABBITMQ_USER/RABBITMQ_PASSWORD are required in $SECRETS_FILE" >&2
  exit 1
fi

if docker exec "$RABBIT_CONTAINER" rabbitmqctl list_users \
  | awk 'NR>1 {print $1}' | grep -Fxq "$rabbit_user"; then
  docker exec "$RABBIT_CONTAINER" rabbitmqctl change_password "$rabbit_user" "$rabbit_password" >/dev/null
else
  docker exec "$RABBIT_CONTAINER" rabbitmqctl add_user "$rabbit_user" "$rabbit_password" >/dev/null
fi
docker exec "$RABBIT_CONTAINER" rabbitmqctl set_permissions -p / "$rabbit_user" '.*' '.*' '.*' >/dev/null

timestamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="${CONFIG_ROOT}.rabbit-backup-${timestamp}"
mkdir -p "$backup_dir"

mapfile -t configs < <(grep -Rl '^[[:space:]]*RabbitMQ:' "$CONFIG_ROOT" --include='*.yaml' | sort)
for config in "${configs[@]}"; do
  relative="${config#${CONFIG_ROOT}/}"
  mkdir -p "$backup_dir/$(dirname "$relative")"
  cp -a "$config" "$backup_dir/$relative"

  RABBIT_USER="$rabbit_user" RABBIT_PASSWORD="$rabbit_password" python3 - "$config" <<'PY'
import os
import pathlib
import re
import sys

path = pathlib.Path(sys.argv[1])
text = path.read_text()
start = text.find("RabbitMQ:\n")
if start < 0:
    raise SystemExit(0)
end = text.find("\n\n", start)
if end < 0:
    end = len(text)
block = text[start:end]
block = re.sub(r"(?m)^(\s*User:)\s*.*$", lambda m: f'{m.group(1)} "{os.environ["RABBIT_USER"]}"', block)
block = re.sub(r"(?m)^(\s*Password:)\s*.*$", lambda m: f'{m.group(1)} "{os.environ["RABBIT_PASSWORD"]}"', block)
path.write_text(text[:start] + block + text[end:])
PY
done

services=(
  askxuan-audit-service askxuan-booking-service askxuan-diy-service
  askxuan-finance-service askxuan-logistics-service askxuan-master-service
  askxuan-message-service askxuan-order-service askxuan-payment-service
  askxuan-review-service askxuan-temple-service
)
docker restart "${services[@]}" >/dev/null

for _ in $(seq 1 30); do
  if docker exec "$RABBIT_CONTAINER" rabbitmqctl authenticate_user "$rabbit_user" "$rabbit_password" \
      2>/dev/null | grep -q 'Success'; then
    echo "RabbitMQ credentials synchronized for ${#configs[@]} configs"
    echo "Backup: $backup_dir"
    exit 0
  fi
  sleep 1
done

echo "RabbitMQ authentication check failed" >&2
exit 1
