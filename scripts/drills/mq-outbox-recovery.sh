#!/usr/bin/env bash
set -euo pipefail

if [[ "${DRILL_CONFIRM:-}" != "YES" ]]; then
  echo "Refusing to stop RabbitMQ. Run with DRILL_CONFIRM=YES in a test environment." >&2
  exit 2
fi

event_key="drill:$(date +%s)"
CONFIG_ROOT="${CONFIG_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.docker/etc}"
SECRETS_ENV="${SECRETS_ENV:-/opt/askxuan/runtime/secrets.env}"
if [[ -f "$SECRETS_ENV" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$SECRETS_ENV"
  set +a
fi
payment_config="$CONFIG_ROOT/payment/payment.yaml"
payment_dsn="$(awk '/DataSource:/ {sub(/^[^:]*:[[:space:]]*/, ""); gsub(/^"|"$/, ""); print; exit}' "$payment_config")"
payment_auth="${payment_dsn%%@tcp(*}"
payment_user="${payment_auth%%:*}"
payment_password="${APP_DB_PASSWORD:-${payment_auth#*:}}"
payment_database="${payment_dsn#*/}"
payment_database="${payment_database%%\?*}"
restore() { docker start askxuan-rabbitmq >/dev/null 2>&1 || true; }
trap restore EXIT

docker stop askxuan-rabbitmq >/dev/null
docker exec -i -e MYSQL_PWD="$payment_password" askxuan-mysql mysql -h127.0.0.1 -u"$payment_user" "$payment_database" <<SQL
INSERT INTO event_outbox(event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at)
VALUES('$event_key','drill','$event_key','drill.probe','ops.drill.events','',JSON_OBJECT('eventKey','$event_key'),'pending',0,NOW());
SQL
sleep 5
status="$(docker exec -e MYSQL_PWD="$payment_password" askxuan-mysql mysql -h127.0.0.1 -N -u"$payment_user" "$payment_database" -e "SELECT status FROM event_outbox WHERE event_key='$event_key'")"
[[ "$status" == "pending" || "$status" == "processing" ]] || { echo "expected pending while RabbitMQ is down, got $status" >&2; exit 1; }

docker start askxuan-rabbitmq >/dev/null
for _ in {1..30}; do
  status="$(docker exec -e MYSQL_PWD="$payment_password" askxuan-mysql mysql -h127.0.0.1 -N -u"$payment_user" "$payment_database" -e "SELECT status FROM event_outbox WHERE event_key='$event_key'")"
  [[ "$status" == "sent" ]] && break
  sleep 2
done
[[ "$status" == "sent" ]] || { echo "outbox did not recover, status=$status" >&2; exit 1; }
echo "PASS RabbitMQ outage retained and later delivered $event_key"
