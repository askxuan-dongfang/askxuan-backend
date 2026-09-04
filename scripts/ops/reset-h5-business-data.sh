#!/usr/bin/env bash
# Reset H5-visible business data while preserving customer/master/admin accounts
# and the reference catalogs required to keep the application usable.

set -euo pipefail

if [ "${CONFIRM:-}" != "RESET_H5_BUSINESS_DATA" ]; then
  echo "Refusing to run. Set CONFIRM=RESET_H5_BUSINESS_DATA after reviewing this script." >&2
  exit 2
fi

MYSQL_CONTAINER="${MYSQL_CONTAINER:-askxuan-mysql}"
APP_REDIS_CONTAINER="${APP_REDIS_CONTAINER:-askxuan-redis}"
RABBITMQ_CONTAINER="${RABBITMQ_CONTAINER:-askxuan-rabbitmq}"
OPENIM_MONGO_CONTAINER="${OPENIM_MONGO_CONTAINER:-mongo}"
OPENIM_REDIS_CONTAINER="${OPENIM_REDIS_CONTAINER:-redis}"
MINIO_CONTAINER="${MINIO_CONTAINER:-askxuan-minio}"
PROJECT_ROOT="${PROJECT_ROOT:-/opt/askxuan/backend}"
SECRETS_FILE="${SECRETS_FILE:-/opt/askxuan/runtime/secrets.env}"
BACKUP_ROOT="${BACKUP_ROOT:-/opt/askxuan/backups}"
OPENIM_ROOT="${OPENIM_ROOT:-$PROJECT_ROOT/.local/openim/open-im-server-3.8.3}"
OPENIM_BIN_DIR="$OPENIM_ROOT/_output/bin/platforms/linux/amd64"
OPENIM_CONFIG_DIR="$OPENIM_ROOT/config/"
OPENIM_LOG_DIR="$OPENIM_ROOT/_output/logs"

secret_value() {
  local key="$1"
  sed -n "s/^${key}=//p" "$SECRETS_FILE" | tail -1
}

container_env_value() {
  local container="$1"
  local key="$2"
  docker inspect "$container" --format '{{range .Config.Env}}{{println .}}{{end}}' \
    | sed -n "s/^${key}=//p" | head -1
}

command_option_value() {
  local container="$1"
  local option="$2"
  docker inspect "$container" --format '{{range .Config.Cmd}}{{println .}}{{end}}' \
    | awk -v option="$option" 'take { print; exit } $0 == option { take=1 }'
}

if [ ! -r "$SECRETS_FILE" ]; then
  echo "Secrets file is not readable: $SECRETS_FILE" >&2
  exit 1
fi

mysql_password="$(secret_value MYSQL_ROOT_PASSWORD)"
openim_mongo_password="$(container_env_value "$OPENIM_MONGO_CONTAINER" MONGO_INITDB_ROOT_PASSWORD)"
openim_redis_password="$(command_option_value "$OPENIM_REDIS_CONTAINER" --requirepass)"
openim_secret="$(secret_value OPENIM_SECRET)"
minio_access="$(secret_value MINIO_ACCESS_KEY)"
minio_secret="$(secret_value MINIO_SECRET_KEY)"

for credential in mysql_password openim_mongo_password openim_redis_password openim_secret minio_access minio_secret; do
  if [ -z "${!credential}" ]; then
    echo "Required credential is unavailable: $credential" >&2
    exit 1
  fi
done

timestamp="$(date +%Y%m%d-%H%M%S)"
backup_dir="$BACKUP_ROOT/h5-business-reset-$timestamp"
mkdir -p "$backup_dir"
chmod 700 "$backup_dir"

service_containers=()
while IFS= read -r container; do
  [ -n "$container" ] && service_containers+=("$container")
done < <(docker ps --format '{{.Names}}' | grep -E '^askxuan-.*-service$' | sort)

openim_was_running=0
openim_pids=()
while IFS= read -r pid; do
  [ -n "$pid" ] && openim_pids+=("$pid")
done < <(pgrep -f "^${OPENIM_ROOT}/_output/bin/platforms/linux/amd64/openim-" || true)
if [ "${#openim_pids[@]}" -gt 0 ]; then
  openim_was_running=1
fi

openim_ready() {
  local response
  response="$(curl -fsS --max-time 5 -X POST http://127.0.0.1:10002/auth/get_admin_token \
    -H 'Content-Type: application/json' \
    -H 'operationID: askxuan-h5-reset-ready' \
    -d "{\"secret\":\"$openim_secret\",\"userID\":\"imAdmin\"}" 2>/dev/null)" || return 1
  printf '%s' "$response" | grep -Eq '"errCode"[[:space:]]*:[[:space:]]*0'
}

ensure_openim_running() {
  local binaries=(
    openim-rpc-user openim-rpc-friend openim-rpc-group openim-rpc-auth
    openim-rpc-conversation openim-rpc-msg openim-rpc-third
    openim-msgtransfer openim-push openim-msggateway openim-api openim-crontask
  )
  local binary
  mkdir -p "$OPENIM_LOG_DIR"
  for binary in "${binaries[@]}"; do
    if ! pgrep -f "^${OPENIM_BIN_DIR}/${binary} " >/dev/null; then
      nohup "$OPENIM_BIN_DIR/$binary" -i 0 -c "$OPENIM_CONFIG_DIR" \
        </dev/null >>"$OPENIM_LOG_DIR/$binary-h5-reset.log" 2>&1 &
    fi
  done
  for _ in $(seq 1 60); do
    openim_ready && return 0
    sleep 2
  done
  echo "OpenIM did not become ready after direct binary startup." >&2
  return 1
}

runtime_restored=0
restore_runtime() {
  if [ "$openim_was_running" = "1" ]; then
    ensure_openim_running
  fi
  if [ "${#service_containers[@]}" -gt 0 ]; then
    docker start "${service_containers[@]}" >/dev/null
  fi
}

on_exit() {
  local status=$?
  trap - EXIT
  if [ "$runtime_restored" != "1" ]; then
    echo "Reset interrupted; restoring stopped application runtimes." >&2
    restore_runtime || true
  fi
  exit "$status"
}
trap on_exit EXIT

printf 'Backup directory: %s\n' "$backup_dir"
printf 'Stopping %d askXuan service containers.\n' "${#service_containers[@]}"
if [ "${#service_containers[@]}" -gt 0 ]; then
  docker stop -t 30 "${service_containers[@]}" >/dev/null
fi

if [ "$openim_was_running" = "1" ]; then
  printf 'Stopping %d OpenIM processes.\n' "${#openim_pids[@]}"
  kill -TERM "${openim_pids[@]}"
  for _ in $(seq 1 15); do
    remaining=0
    for pid in "${openim_pids[@]}"; do
      if kill -0 "$pid" 2>/dev/null; then
        remaining=1
      fi
    done
    [ "$remaining" = "0" ] && break
    sleep 1
  done
  for pid in "${openim_pids[@]}"; do
    if kill -0 "$pid" 2>/dev/null; then
      echo "OpenIM process did not stop: $pid" >&2
      exit 1
    fi
  done
fi

mysql_databases=()
while IFS= read -r database; do
  [ -n "$database" ] && mysql_databases+=("$database")
done < <(docker exec -e MYSQL_PWD="$mysql_password" "$MYSQL_CONTAINER" \
  mysql -uroot -N -B -e "SELECT schema_name FROM information_schema.schemata WHERE schema_name LIKE 'askxuan%' ORDER BY schema_name")

printf 'Backing up %d MySQL databases.\n' "${#mysql_databases[@]}"
docker exec -e MYSQL_PWD="$mysql_password" "$MYSQL_CONTAINER" \
  mysqldump -uroot --single-transaction --quick --routines --triggers --events \
  --databases "${mysql_databases[@]}" \
  | gzip -1 >"$backup_dir/mysql-askxuan.sql.gz"

printf 'Backing up OpenIM MongoDB.\n'
docker exec "$OPENIM_MONGO_CONTAINER" mongodump --quiet \
  -u root -p "$openim_mongo_password" --authenticationDatabase admin \
  --db openim_v3 --archive \
  | gzip -1 >"$backup_dir/openim-mongo.archive.gz"

printf 'Backing up application and OpenIM Redis.\n'
docker exec "$APP_REDIS_CONTAINER" redis-cli SAVE >/dev/null
docker cp "$APP_REDIS_CONTAINER:/data/dump.rdb" "$backup_dir/askxuan-redis.rdb" >/dev/null
docker exec -e REDISCLI_AUTH="$openim_redis_password" "$OPENIM_REDIS_CONTAINER" redis-cli SAVE >/dev/null
docker cp "$OPENIM_REDIS_CONTAINER:/data/dump.rdb" "$backup_dir/openim-redis.rdb" >/dev/null

minio_data_source="$(docker inspect "$MINIO_CONTAINER" --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{println .Source}}{{end}}{{end}}')"
if [ -z "$minio_data_source" ] || [ ! -d "$minio_data_source/askxuan-media" ]; then
  echo "Unable to locate the askxuan-media bucket directory." >&2
  exit 1
fi
printf 'Backing up the askxuan-media bucket.\n'
tar -C "$minio_data_source" -czf "$backup_dir/askxuan-media.tar.gz" askxuan-media

for backup in \
  "$backup_dir/mysql-askxuan.sql.gz" \
  "$backup_dir/openim-mongo.archive.gz" \
  "$backup_dir/askxuan-redis.rdb" \
  "$backup_dir/openim-redis.rdb" \
  "$backup_dir/askxuan-media.tar.gz"; do
  if [ ! -s "$backup" ]; then
    echo "Backup is empty: $backup" >&2
    exit 1
  fi
done
sha256sum "$backup_dir"/* >"$backup_dir/SHA256SUMS"

printf 'Purging any queued business events after consumers stopped.\n'
while read -r queue ready unacked; do
  [ "$queue" = "name" ] && continue
  if [ "${ready:-0}" -gt 0 ] || [ "${unacked:-0}" -gt 0 ]; then
    docker exec "$RABBITMQ_CONTAINER" rabbitmqctl purge_queue "$queue" >/dev/null
  fi
done < <(docker exec "$RABBITMQ_CONTAINER" rabbitmqctl list_queues -q name messages_ready messages_unacknowledged)

printf 'Clearing MySQL business and user-generated data.\n'
docker exec -e MYSQL_PWD="$mysql_password" -i "$MYSQL_CONTAINER" \
  mysql -uroot --default-character-set=utf8mb4 <<'SQL'
SET FOREIGN_KEY_CHECKS=0;

TRUNCATE TABLE askxuan.message;
TRUNCATE TABLE askxuan.booking;

TRUNCATE TABLE askxuan_booking.event_outbox;
TRUNCATE TABLE askxuan_booking.booking_chat_message;
TRUNCATE TABLE askxuan_booking.booking_review;
TRUNCATE TABLE askxuan_booking.booking_status_log;
TRUNCATE TABLE askxuan_booking.booking_slot_inventory;
TRUNCATE TABLE askxuan_booking.consultation_order;
TRUNCATE TABLE askxuan_booking.booking;

TRUNCATE TABLE askxuan_diy.event_outbox;
TRUNCATE TABLE askxuan_diy.diy_creator_earning;
TRUNCATE TABLE askxuan_diy.blessing_task;
TRUNCATE TABLE askxuan_diy.diy_order_item;
TRUNCATE TABLE askxuan_diy.diy_order;
TRUNCATE TABLE askxuan_diy.diy_design;

TRUNCATE TABLE askxuan_order.outbox;
TRUNCATE TABLE askxuan_order.return_order;
TRUNCATE TABLE askxuan_order.shop_order_logistics;
TRUNCATE TABLE askxuan_order.shop_order_item;
TRUNCATE TABLE askxuan_order.shop_order;

TRUNCATE TABLE askxuan_shop.refund;
TRUNCATE TABLE askxuan_shop.payment_log;
TRUNCATE TABLE askxuan_shop.payment;
TRUNCATE TABLE askxuan_shop.return_order;
TRUNCATE TABLE askxuan_shop.shop_order_logistics;
TRUNCATE TABLE askxuan_shop.shop_order_item;
TRUNCATE TABLE askxuan_shop.shop_order;

TRUNCATE TABLE askxuan_payment.event_outbox;
TRUNCATE TABLE askxuan_payment.refund;
TRUNCATE TABLE askxuan_payment.payment_log;
TRUNCATE TABLE askxuan_payment.payment;

TRUNCATE TABLE askxuan_finance.finance_ledger_entry;
TRUNCATE TABLE askxuan_finance.finance_log;
TRUNCATE TABLE askxuan_finance.settlement;
TRUNCATE TABLE askxuan_finance.withdrawal;
TRUNCATE TABLE askxuan_finance.finance_transaction;

TRUNCATE TABLE askxuan_master.master_earning;
TRUNCATE TABLE askxuan_master.master_audit;
TRUNCATE TABLE askxuan_temple.temple_audit;

TRUNCATE TABLE askxuan_review.review_report;
TRUNCATE TABLE askxuan_review.review_reply;
TRUNCATE TABLE askxuan_review.review;

TRUNCATE TABLE askxuan_audit.audit_log;
TRUNCATE TABLE askxuan_audit.audit_queue;
TRUNCATE TABLE askxuan_audit.report;

TRUNCATE TABLE askxuan_message.push_log;
TRUNCATE TABLE askxuan_message.message;
TRUNCATE TABLE askxuan_message.system_announcement;

TRUNCATE TABLE askxuan_ai.ai_tool_call;
TRUNCATE TABLE askxuan_ai.ai_usage_log;
TRUNCATE TABLE askxuan_ai.ai_run;
TRUNCATE TABLE askxuan_ai.ai_message;
TRUNCATE TABLE askxuan_ai.ai_usage_counter;
TRUNCATE TABLE askxuan_ai.ai_session;

TRUNCATE TABLE askxuan_community.post_asset;
TRUNCATE TABLE askxuan_community.post_like;
TRUNCATE TABLE askxuan_community.post_comment;
TRUNCATE TABLE askxuan_community.master_follow;
TRUNCATE TABLE askxuan_community.post;

TRUNCATE TABLE askxuan_media.live_room;
TRUNCATE TABLE askxuan_media.media_asset;
TRUNCATE TABLE askxuan_marketing.coupon_record;
TRUNCATE TABLE askxuan_product.product_stock_reservation;
TRUNCATE TABLE askxuan_product.product_favorite;
TRUNCATE TABLE askxuan_temple.temple_favorite;
TRUNCATE TABLE askxuan_logistics.logistics_track;
TRUNCATE TABLE askxuan_system.operation_log;

UPDATE askxuan_user.user_profile
SET total_orders=0,total_spent=0.00,update_time=CURRENT_TIMESTAMP;

SET FOREIGN_KEY_CHECKS=1;
SQL

if [ -r "$PROJECT_ROOT/scripts/db/20260828_diy_eastern_material_catalog.sql" ]; then
  printf 'Restoring the authoritative DIY material and stock baseline.\n'
  docker exec -e MYSQL_PWD="$mysql_password" -i "$MYSQL_CONTAINER" \
    mysql -uroot --default-character-set=utf8mb4 askxuan_diy \
    <"$PROJECT_ROOT/scripts/db/20260828_diy_eastern_material_catalog.sql"
fi

printf 'Clearing application Redis transient business state.\n'
docker exec "$APP_REDIS_CONTAINER" redis-cli FLUSHDB >/dev/null

printf 'Clearing OpenIM message, sequence and conversation state while preserving users.\n'
docker exec "$OPENIM_MONGO_CONTAINER" mongosh --quiet \
  -u root -p "$openim_mongo_password" --authenticationDatabase admin openim_v3 \
  --eval 'for (const name of ["conversation","conversation_version","msg","seq","seq_user"]) { db.getCollection(name).deleteMany({}); }' >/dev/null

docker exec -e REDISCLI_AUTH="$openim_redis_password" "$OPENIM_REDIS_CONTAINER" sh -lc '
  for pattern in \
    "CONVERSATION_IDS:*" "CONVERSATION:*" "CONVERSATION_USER_MAX:*" \
    "MALLOC_SEQ:*" "MSG_CACHE:*" "SEND_MSG_FAILED_FLAG:*" \
    "SEQ_USER_MIN:*" "SEQ_USER_READ:*"; do
    redis-cli --scan --pattern "$pattern" | xargs -r -n 100 redis-cli UNLINK >/dev/null
  done
'

printf 'Clearing H5-created media objects.\n'
docker exec \
  -e MC_CONFIG_DIR="/tmp/askxuan-h5-reset-$timestamp" \
  -e MINIO_ACCESS="$minio_access" \
  -e MINIO_SECRET="$minio_secret" \
  "$MINIO_CONTAINER" sh -lc '
    mc alias set local http://127.0.0.1:9000 "$MINIO_ACCESS" "$MINIO_SECRET" >/dev/null
    mc rm --recursive --force local/askxuan-media >/dev/null
  '

printf 'Restoring OpenIM and askXuan services.\n'
restore_runtime
runtime_restored=1

printf 'Reset completed. Backup: %s\n' "$backup_dir"
trap - EXIT
