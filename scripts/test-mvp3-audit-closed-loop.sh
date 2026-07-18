#!/bin/bash
# ============================================================
# MVP-3 审核服务闭环测试
# 前置：启动 audit-service（端口 8093）
# ============================================================

set -o pipefail

BASE=http://localhost:8093
PASS_COUNT=0
FAIL_COUNT=0
TOTAL=10

# 颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

# 检查 jq 是否安装
if ! command -v jq &> /dev/null; then
    echo "错误：需要 jq 工具，请执行 brew install jq"
    exit 1
fi

info "开始 MVP-3 审核闭环测试（共 $TOTAL 步）"
echo "================================================"

# ===== 0. 恢复固定数据库数据并重启容器，确保幂等 =====
info "步骤 0: 恢复审核测试数据并重启 audit-service"
docker exec askxuan-mysql mysql -uroot -proot123 askxuan_audit -e "
INSERT INTO audit_queue (id,biz_type,biz_id,submitter_id,content_snapshot,status,auditor_id,audit_time,audit_remark)
VALUES
  (900001,'design','TEST-DESIGN-001','1','{\"name\":\"审核测试设计\"}','pending','',NULL,''),
  (900002,'comment','TEST-COMMENT-001','2','{\"content\":\"审核测试评论\"}','pending','',NULL,''),
  (900003,'master','TEST-MASTER-001','T001','{\"name\":\"审核测试法师\"}','approved','A001',NOW(),'历史测试通过'),
  (900004,'temple','TEST-TEMPLE-001','T001','{\"name\":\"审核测试寺院\"}','approved','A001',NOW(),'历史测试通过')
ON DUPLICATE KEY UPDATE status=VALUES(status),auditor_id=VALUES(auditor_id),audit_time=VALUES(audit_time),audit_remark=VALUES(audit_remark);
INSERT INTO report (id,reporter_id,target_type,target_id,reason,evidence_urls,status,handler_id,handle_result)
VALUES
  (900001,'1','design','TEST-REPORT-001','测试举报','[]','pending','',''),
  (900002,'2','comment','TEST-REPORT-002','历史举报','[]','handled','A001','已处理')
ON DUPLICATE KEY UPDATE status=VALUES(status),handler_id=VALUES(handler_id),handle_result=VALUES(handle_result);
INSERT INTO sensitive_word (word,category,status) VALUES
  ('邪教','religious','enabled'),('反动','political','enabled'),('色情','vulgar','enabled'),('加微信','advertising','enabled'),('代购','advertising','disabled')
ON DUPLICATE KEY UPDATE category=VALUES(category),status=VALUES(status);
DELETE FROM sensitive_word WHERE word='测试敏感词';
" >/dev/null
docker restart askxuan-audit-service >/dev/null
READY=0
for _ in $(seq 1 30); do
    if curl -fsS "$BASE/api/v1/admin/audit/statistics" >/dev/null 2>&1; then
        READY=1
        break
    fi
    sleep 1
done
if [ "$READY" != "1" ]; then
    echo "错误：audit-service 重启后未就绪"
    exit 1
fi

AUDIT_ID=""
REJECT_ID=""
REPORT_ID=""
SW_ID=""
NEW_SW_ID=""

# ===== 1. 审核队列 =====
info "步骤 1/$TOTAL: 审核队列"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/audit/queue")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
# 取第一个 pending 状态的 id
FIRST_PENDING=$(echo "$RESP" | jq -r '.data.list[]? | select(.status=="pending") | .id' | head -n1)
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 4 ] 2>/dev/null && [ -n "$FIRST_PENDING" ]; then
    AUDIT_ID=$FIRST_PENDING
    pass "审核队列查询成功, total=$TOTAL_COUNT, 第一个 pending id=$AUDIT_ID"
else
    fail "审核队列查询失败: $RESP"
fi

# ===== 2. 审核详情 =====
info "步骤 2/$TOTAL: 审核详情"
if [ -n "$AUDIT_ID" ]; then
    RESP=$(curl -s -X GET "$BASE/api/v1/admin/audit/queue/$AUDIT_ID")
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    BIZ_TYPE=$(echo "$RESP" | jq -r '.data.bizType // empty')
    if [ "$CODE" = "0" ] && [ -n "$BIZ_TYPE" ] && [ "$BIZ_TYPE" != "null" ]; then
        pass "审核详情成功, bizType=$BIZ_TYPE"
    else
        fail "审核详情失败: $RESP"
    fi
else
    fail "审核详情跳过：AUDIT_ID 为空"
fi

# ===== 3. 通过审核 =====
info "步骤 3/$TOTAL: 通过审核 AUDIT_ID"
if [ -n "$AUDIT_ID" ]; then
    RESP=$(curl -s -X PUT "$BASE/api/v1/admin/audit/queue/$AUDIT_ID/approve" \
        -H 'Content-Type: application/json' \
        -d '{"auditorId":"A001","remark":"通过"}')
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    STATUS=$(echo "$RESP" | jq -r '.data.status // empty')
    if [ "$CODE" = "0" ] && [ "$STATUS" = "approved" ]; then
        pass "通过审核成功, status=$STATUS"
    else
        fail "通过审核失败: $RESP"
    fi
else
    fail "通过审核跳过：AUDIT_ID 为空"
fi

# ===== 4. 驳回另一条审核 =====
info "步骤 4/$TOTAL: 查找另一条 pending 并驳回"
# 重新拉取列表，找到另一个 pending（排除已审批的 AUDIT_ID）
RESP=$(curl -s -X GET "$BASE/api/v1/admin/audit/queue")
REJECT_ID=$(echo "$RESP" | jq -r --arg used "$AUDIT_ID" '.data.list[]? | select(.status=="pending") | select(.id != ($used | tonumber)) | .id' | head -n1)
if [ -n "$REJECT_ID" ]; then
    RESP=$(curl -s -X PUT "$BASE/api/v1/admin/audit/queue/$REJECT_ID/reject" \
        -H 'Content-Type: application/json' \
        -d '{"auditorId":"A001","remark":"内容不合规"}')
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    STATUS=$(echo "$RESP" | jq -r '.data.status // empty')
    if [ "$CODE" = "0" ] && [ "$STATUS" = "rejected" ]; then
        pass "驳回审核成功, id=$REJECT_ID, status=$STATUS"
    else
        fail "驳回审核失败: $RESP"
    fi
else
    fail "驳回审核跳过：找不到第二个 pending 记录"
fi

# ===== 5. 举报列表 =====
info "步骤 5/$TOTAL: 举报列表"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/audit/reports")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
FIRST_PENDING_RPT=$(echo "$RESP" | jq -r '.data.list[]? | select(.status=="pending") | .id' | head -n1)
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 2 ] 2>/dev/null && [ -n "$FIRST_PENDING_RPT" ]; then
    REPORT_ID=$FIRST_PENDING_RPT
    pass "举报列表查询成功, total=$TOTAL_COUNT, 第一个 pending id=$REPORT_ID"
else
    fail "举报列表查询失败: $RESP"
fi

# ===== 6. 处理举报 =====
info "步骤 6/$TOTAL: 处理举报 REPORT_ID"
if [ -n "$REPORT_ID" ]; then
    RESP=$(curl -s -X PUT "$BASE/api/v1/admin/audit/reports/$REPORT_ID/handle" \
        -H 'Content-Type: application/json' \
        -d '{"handlerId":"A001","handleResult":"handled"}')
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    STATUS=$(echo "$RESP" | jq -r '.data.status // empty')
    if [ "$CODE" = "0" ] && [ "$STATUS" = "handled" ]; then
        pass "处理举报成功, status=$STATUS"
    else
        fail "处理举报失败: $RESP"
    fi
else
    fail "处理举报跳过：REPORT_ID 为空"
fi

# ===== 7. 敏感词列表 =====
info "步骤 7/$TOTAL: 敏感词列表"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/audit/sensitive-words")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
# 取列表最后一个 id 作为 SW_ID
LAST_SW=$(echo "$RESP" | jq -r '.data.list[-1].id // empty')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 5 ] 2>/dev/null && [ -n "$LAST_SW" ]; then
    SW_ID=$LAST_SW
    pass "敏感词列表查询成功, total=$TOTAL_COUNT, 最后一个 id=$SW_ID"
else
    fail "敏感词列表查询失败: $RESP"
fi

# ===== 8. 创建敏感词 =====
info "步骤 8/$TOTAL: 创建敏感词"
RESP=$(curl -s -X POST "$BASE/api/v1/admin/audit/sensitive-words" \
    -H 'Content-Type: application/json' \
    -d '{"word":"测试敏感词","category":"vulgar"}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
ID=$(echo "$RESP" | jq -r '.data.id // empty')
if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
    NEW_SW_ID=$ID
    pass "创建敏感词成功, id=$NEW_SW_ID"
else
    fail "创建敏感词失败: $RESP"
fi

# ===== 9. 删除敏感词 =====
info "步骤 9/$TOTAL: 删除敏感词 NEW_SW_ID"
if [ -n "$NEW_SW_ID" ]; then
    RESP=$(curl -s -X DELETE "$BASE/api/v1/admin/audit/sensitive-words/$NEW_SW_ID")
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    if [ "$CODE" = "0" ]; then
        pass "删除敏感词成功, id=$NEW_SW_ID"
    else
        fail "删除敏感词失败: $RESP"
    fi
else
    fail "删除敏感词跳过：NEW_SW_ID 为空"
fi

# ===== 10. 统计 =====
info "步骤 10/$TOTAL: 审核统计"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/audit/statistics")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_CNT=$(echo "$RESP" | jq -r '.data.totalCount // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_CNT" -ge 4 ] 2>/dev/null; then
    pass "审核统计成功, totalCount=$TOTAL_CNT"
else
    fail "审核统计失败: $RESP"
fi

echo "================================================"
echo "测试结果: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT 失败"
[ $FAIL_COUNT -eq 0 ] && exit 0 || exit 1
