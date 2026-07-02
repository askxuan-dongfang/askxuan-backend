#!/bin/bash
# askXuan-backend 数据库迁移脚本
# 使用方式：./scripts/migrate.sh [up|down|status]
# 依赖：Docker 容器 askxuan-mysql 已启动

set -e

ACTION=${1:-up}
MYSQL_CONTAINER="askxuan-mysql"
MYSQL_USER="root"
MYSQL_PASS="root123"
SQL_FILE="db/init.sql"

cd "$(dirname "$0")/.."

case "$ACTION" in
    up)
        echo "==> 执行数据库初始化（建表 + 种子数据）..."
        if ! docker ps --format '{{.Names}}' | grep -q "^${MYSQL_CONTAINER}$"; then
            echo "ERROR: MySQL 容器 ${MYSQL_CONTAINER} 未运行，请先执行 docker compose up -d"
            exit 1
        fi
        docker exec -i "${MYSQL_CONTAINER}" mysql -u"${MYSQL_USER}" -p"${MYSQL_PASS}" < "${SQL_FILE}"
        echo "==> 数据库初始化完成"
        ;;
    down)
        echo "==> 警告：将清除所有 askxuan_* 数据库！"
        read -p "确认清除？(yes/no): " confirm
        if [ "$confirm" = "yes" ]; then
            docker exec -i "${MYSQL_CONTAINER}" mysql -u"${MYSQL_USER}" -p"${MYSQL_PASS}" -e "
                SET FOREIGN_KEY_CHECKS = 0;
                SELECT CONCAT('DROP DATABASE ', schema_name, ';') 
                FROM information_schema.schemata 
                WHERE schema_name LIKE 'askxuan_%';
                SET FOREIGN_KEY_CHECKS = 1;
            " | grep "DROP DATABASE" | docker exec -i "${MYSQL_CONTAINER}" mysql -u"${MYSQL_USER}" -p"${MYSQL_PASS}"
            echo "==> 数据库已清除"
        else
            echo "==> 已取消"
        fi
        ;;
    status)
        echo "==> 数据库状态："
        docker exec "${MYSQL_CONTAINER}" mysql -u"${MYSQL_USER}" -p"${MYSQL_PASS}" -e "
            SELECT table_schema AS '数据库', COUNT(*) AS '表数量' 
            FROM information_schema.tables 
            WHERE table_schema LIKE 'askxuan_%' 
            GROUP BY table_schema;
        " 2>/dev/null
        ;;
    *)
        echo "Usage: $0 [up|down|status]"
        exit 1
        ;;
esac
