#!/usr/bin/env bash
set -euo pipefail
REPORT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPORT_CONTAINER="askxuan-report-test-$$"
trap 'docker rm -f "$REPORT_CONTAINER" >/dev/null 2>&1 || true' EXIT
docker run -d --rm --name "$REPORT_CONTAINER" --tmpfs /var/lib/mysql:rw,size=1g -e MYSQL_ALLOW_EMPTY_PASSWORD=yes -e MYSQL_DATABASE=askxuan -e MYSQL_INITDB_SKIP_TZINFO=1 -p 127.0.0.1::3306 mysql:8 --innodb-flush-log-at-trx-commit=0 --sync-binlog=0 >/dev/null
for ((i=0;i<90;i++));do if docker exec "$REPORT_CONTAINER" mysqladmin --protocol=tcp -h127.0.0.1 ping >/dev/null 2>&1;then break;fi;sleep 1;done
docker exec -i "$REPORT_CONTAINER" mysql --protocol=tcp -h127.0.0.1 askxuan < "$REPORT_ROOT/db/init.sql"
docker exec -i "$REPORT_CONTAINER" mysql --protocol=tcp -h127.0.0.1 < "$REPORT_ROOT/scripts/db/20260909_ai_topic_reports.sql"
# Reapplying this migration must preserve orders, prices and existing data.
docker exec -i "$REPORT_CONTAINER" mysql --protocol=tcp -h127.0.0.1 < "$REPORT_ROOT/scripts/db/20260909_ai_topic_reports.sql"
REPORT_PORT="$(docker port "$REPORT_CONTAINER" 3306/tcp)";REPORT_PORT="${REPORT_PORT##*:}"
# Exercise the production service privilege boundary, never a root-only happy path.
docker exec -i "$REPORT_CONTAINER" mysql --protocol=tcp -h127.0.0.1 <<'SQL'
CREATE USER 'ai_user'@'%';
CREATE USER 'payment_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_ai.* TO 'ai_user'@'%';
GRANT ALL PRIVILEGES ON askxuan_payment.* TO 'payment_user'@'%';
SQL
docker exec -i "$REPORT_CONTAINER" mysql --protocol=tcp -h127.0.0.1 < "$REPORT_ROOT/scripts/db/20260909_ai_topic_report_permissions.sql"
docker exec -i "$REPORT_CONTAINER" mysql --protocol=tcp -h127.0.0.1 < "$REPORT_ROOT/scripts/db/20260909_ai_topic_report_permissions.sql"
export AI_REPORT_TEST_DSN="ai_user@tcp(127.0.0.1:${REPORT_PORT})/askxuan_ai?charset=utf8mb4&timeout=5s"
export REPORT_PAYMENT_TEST_DSN="payment_user@tcp(127.0.0.1:${REPORT_PORT})/askxuan_payment?charset=utf8mb4&timeout=5s"
cd "$REPORT_ROOT"
go test ./services/infrastructure/ai-service/internal/logic -run TestMySQLReportGenerate -count=1
go test ./services/commerce/payment-service/internal/handler -run TestMySQLReportPurchase -count=1
go test ./services/infrastructure/ai-service/internal/logic -run TestMySQLReportReadPaid -count=1
