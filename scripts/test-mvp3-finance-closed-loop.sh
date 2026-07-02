#!/bin/bash
# ============================================================
# MVP-3 财务服务闭环测试
# 前置：启动 finance-service（端口 8091）
# ============================================================

set -o pipefail

BASE=http://localhost:8091
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

info "开始 MVP-3 财务闭环测试（共 $TOTAL 步）"
echo "================================================"

# ===== 1. 总览 =====
info "步骤 1/$TOTAL: 财务总览"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/finance/overview")
CODE=$(echo "$RESP" | jq -r '.code // 0')
PENDING=$(echo "$RESP" | jq -r '.data.pendingWithdraw // 0')
if [ "$CODE" = "0" ] && [ "$PENDING" -ge 0 ] 2>/dev/null; then
    pass "财务总览成功, pendingWithdraw=$PENDING"
else
    fail "财务总览失败: $RESP"
fi

# ===== 2. 结算列表 =====
info "步骤 2/$TOTAL: 结算列表"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/finance/settlements")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 3 ] 2>/dev/null; then
    pass "结算列表查询成功, total=$TOTAL_COUNT"
else
    fail "结算列表查询失败: $RESP"
fi

# ===== 3. 结算详情 =====
info "步骤 3/$TOTAL: 结算详情"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/finance/settlements/1")
CODE=$(echo "$RESP" | jq -r '.code // 0')
SETTLE_NO=$(echo "$RESP" | jq -r '.data.settlementNo // empty')
if [ "$CODE" = "0" ] && [ -n "$SETTLE_NO" ] && [ "$SETTLE_NO" != "null" ]; then
    pass "结算详情成功, settlementNo=$SETTLE_NO"
else
    fail "结算详情失败: $RESP"
fi

# ===== 4. 确认结算 =====
info "步骤 4/$TOTAL: 确认结算 id=2"
RESP=$(curl -s -X POST "$BASE/api/v1/admin/finance/settlements/confirm/2")
CODE=$(echo "$RESP" | jq -r '.code // 0')
STATUS=$(echo "$RESP" | jq -r '.data.status // empty')
if [ "$CODE" = "0" ] && [ "$STATUS" = "confirmed" ]; then
    pass "确认结算成功, status=$STATUS"
else
    fail "确认结算失败: $RESP"
fi

# ===== 5. 提现列表 =====
info "步骤 5/$TOTAL: 提现列表"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/finance/withdrawals")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 2 ] 2>/dev/null; then
    pass "提现列表查询成功, total=$TOTAL_COUNT"
else
    fail "提现列表查询失败: $RESP"
fi

# ===== 6. 审核提现 =====
info "步骤 6/$TOTAL: 审核提现 id=1 approve"
RESP=$(curl -s -X PUT "$BASE/api/v1/admin/finance/withdrawals/1/audit" \
    -H 'Content-Type: application/json' \
    -d '{"action":"approve"}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
STATUS=$(echo "$RESP" | jq -r '.data.status // empty')
if [ "$CODE" = "0" ] && [ "$STATUS" = "approved" ]; then
    pass "审核提现成功, status=$STATUS"
else
    fail "审核提现失败: $RESP"
fi

# ===== 7. 打款 =====
info "步骤 7/$TOTAL: 打款 id=1"
RESP=$(curl -s -X PUT "$BASE/api/v1/admin/finance/withdrawals/1/process")
CODE=$(echo "$RESP" | jq -r '.code // 0')
STATUS=$(echo "$RESP" | jq -r '.data.status // empty')
if [ "$CODE" = "0" ] && [ "$STATUS" = "success" ]; then
    pass "打款成功, status=$STATUS"
else
    fail "打款失败: $RESP"
fi

# ===== 8. 抽成配置列表 =====
info "步骤 8/$TOTAL: 抽成配置列表"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/finance/commission-config")
CODE=$(echo "$RESP" | jq -r '.code // 0')
LIST_LEN=$(echo "$RESP" | jq -r '.data.list | length // 0')
if [ "$CODE" = "0" ] && [ "$LIST_LEN" -ge 4 ] 2>/dev/null; then
    pass "抽成配置列表查询成功, list 长度=$LIST_LEN"
else
    fail "抽成配置列表查询失败: $RESP"
fi

# ===== 9. 更新抽成配置 =====
info "步骤 9/$TOTAL: 更新抽成配置 id=1"
RESP=$(curl -s -X PUT "$BASE/api/v1/admin/finance/commission-config/1" \
    -H 'Content-Type: application/json' \
    -d '{"rate":0.12,"description":"测试更新"}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
ID=$(echo "$RESP" | jq -r '.data.id // empty')
if [ "$CODE" = "0" ] && [ "$ID" = "1" ]; then
    pass "更新抽成配置成功, id=$ID"
else
    fail "更新抽成配置失败: $RESP"
fi

# ===== 10. 报表 =====
info "步骤 10/$TOTAL: 报表查询"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/finance/reports?startTime=2026-01-01&endTime=2026-12-31")
CODE=$(echo "$RESP" | jq -r '.code // 0')
if [ "$CODE" = "0" ]; then
    pass "报表查询成功"
else
    fail "报表查询失败: $RESP"
fi

echo "================================================"
echo "测试结果: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT 失败"
[ $FAIL_COUNT -eq 0 ] && exit 0 || exit 1
