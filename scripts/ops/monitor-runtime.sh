#!/usr/bin/env bash
set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:8080/api/v1/health}"
OUTBOX_PENDING_WARN="${OUTBOX_PENDING_WARN:-20}"
OUTBOX_AGE_WARN_SECONDS="${OUTBOX_AGE_WARN_SECONDS:-120}"
ALERT_WEBHOOK_URL="${ALERT_WEBHOOK_URL:-}"
CONFIG_ROOT="${CONFIG_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.docker/etc}"
SECRETS_ENV="${SECRETS_ENV:-/opt/askxuan/runtime/secrets.env}"
if [[ -f "$SECRETS_ENV" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "$SECRETS_ENV"
  set +a
fi
failures=()

fail() { failures+=("$1"); }

if ! curl -fsS --max-time 5 "$GATEWAY_URL" >/dev/null; then fail "gateway health check failed: $GATEWAY_URL"; fi
if ! docker exec askxuan-rabbitmq rabbitmq-diagnostics -q ping >/dev/null 2>&1; then fail "RabbitMQ ping failed"; fi

unhealthy="$(docker ps --filter 'name=askxuan-' --format '{{.Names}} {{.Status}}' | awk '$0 !~ /healthy/ && $0 !~ /askxuan-nginx/ {print}')"
if [[ -n "$unhealthy" ]]; then fail "unhealthy containers: ${unhealthy//$'\n'/, }"; fi

query_outbox() {
  local service="$1" config="$CONFIG_ROOT/$1/$1.yaml" dsn auth user password database
  [[ -f "$config" ]] || return 1
  dsn="$(awk '/DataSource:/ {sub(/^[^:]*:[[:space:]]*/, ""); gsub(/^"|"$/, ""); print; exit}' "$config")"
  [[ "$dsn" == *"@tcp("*"/"* ]] || return 1
  auth="${dsn%%@tcp(*}"
  user="${auth%%:*}"
  password="${APP_DB_PASSWORD:-${auth#*:}}"
  database="${dsn#*/}"
  database="${database%%\?*}"
  docker exec -e MYSQL_PWD="$password" askxuan-mysql mysql -h127.0.0.1 -N -u"$user" "$database" -e \
    "SELECT COUNT(*),SUM(status='dead'),COALESCE(TIMESTAMPDIFF(SECOND,MIN(CASE WHEN status IN ('pending','processing') THEN created_at END),NOW()),0) FROM event_outbox WHERE status IN ('pending','processing','dead')" 2>/dev/null
}

outbox_rows=""
for service in booking payment diy; do
  if row="$(query_outbox "$service")"; then
    outbox_rows+="askxuan_${service} ${row}"$'\n'
  else
    fail "askxuan_${service} outbox metrics query failed"
  fi
done
while read -r db pending dead oldest; do
  [[ -z "${db:-}" ]] && continue
  (( dead > 0 )) && fail "$db has $dead dead outbox events"
  (( pending > OUTBOX_PENDING_WARN )) && fail "$db outbox backlog is $pending"
  (( oldest > OUTBOX_AGE_WARN_SECONDS )) && fail "$db oldest pending outbox is ${oldest}s"
done <<< "$outbox_rows"

if (( ${#failures[@]} > 0 )); then
  message="askXuan runtime alert: $(IFS='; '; echo "${failures[*]}")"
  printf '%s\n' "$message" >&2
  if [[ -n "$ALERT_WEBHOOK_URL" ]]; then curl -fsS -X POST -H 'Content-Type: application/json' --data "{\"text\":\"$message\"}" "$ALERT_WEBHOOK_URL" >/dev/null || true; fi
  exit 1
fi
printf 'OK gateway, RabbitMQ, containers and outbox backlog\n'
