#!/bin/bash
# ============================================================
# MVP-3 评价服务闭环测试
# 前置：启动 review-service（端口 8092）
# ============================================================

set -o pipefail

BASE=http://localhost:8092
PASS_COUNT=0
FAIL_COUNT=0
TOTAL=9

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

info "开始 MVP-3 评价闭环测试（共 $TOTAL 步）"
echo "================================================"

REVIEW_ID=""
REPORT_ID=""

# ===== 1. 创建评价 =====
info "步骤 1/$TOTAL: 创建评价"
RESP=$(curl -s -X POST "$BASE/api/v1/reviews" \
    -H 'Content-Type: application/json' \
    -d '{"userId":"U001","targetType":"booking","targetId":"B001","rating":5,"content":"很好"}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
ID=$(echo "$RESP" | jq -r '.data.id // empty')
if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
    REVIEW_ID=$ID
    pass "创建评价成功, id=$REVIEW_ID"
else
    fail "创建评价失败: $RESP"
fi

# ===== 2. C端列表 =====
info "步骤 2/$TOTAL: C端评价列表"
RESP=$(curl -s -X GET "$BASE/api/v1/reviews?targetType=booking&page=1&size=20")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 1 ] 2>/dev/null; then
    pass "C端列表查询成功, total=$TOTAL_COUNT"
else
    fail "C端列表查询失败: $RESP"
fi

# ===== 3. 评价详情 =====
info "步骤 3/$TOTAL: 评价详情"
if [ -n "$REVIEW_ID" ]; then
    RESP=$(curl -s -X GET "$BASE/api/v1/reviews/$REVIEW_ID")
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    RATING=$(echo "$RESP" | jq -r '.data.rating // 0')
    if [ "$CODE" = "0" ] && [ "$RATING" = "5" ]; then
        pass "评价详情成功, rating=$RATING"
    else
        fail "评价详情失败: $RESP"
    fi
else
    fail "评价详情跳过：REVIEW_ID 为空"
fi

# ===== 4. 管理台列表 =====
info "步骤 4/$TOTAL: 管理台评价列表"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/reviews?status=&page=1&size=20")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 1 ] 2>/dev/null; then
    pass "管理台列表查询成功, total=$TOTAL_COUNT"
else
    fail "管理台列表查询失败: $RESP"
fi

# ===== 5. 管理台详情 =====
info "步骤 5/$TOTAL: 管理台评价详情"
if [ -n "$REVIEW_ID" ]; then
    RESP=$(curl -s -X GET "$BASE/api/v1/admin/reviews/$REVIEW_ID")
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    ID=$(echo "$RESP" | jq -r '.data.id // empty')
    if [ "$CODE" = "0" ] && [ -n "$ID" ]; then
        pass "管理台详情成功, id=$ID"
    else
        fail "管理台详情失败: $RESP"
    fi
else
    fail "管理台详情跳过：REVIEW_ID 为空"
fi

# ===== 6. 回复评价 =====
info "步骤 6/$TOTAL: 回复评价"
if [ -n "$REVIEW_ID" ]; then
    RESP=$(curl -s -X POST "$BASE/api/v1/admin/reviews/$REVIEW_ID/reply" \
        -H 'Content-Type: application/json' \
        -d '{"replierType":"temple_admin","replierId":"A001","content":"感谢评价"}')
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    ID=$(echo "$RESP" | jq -r '.data.id // empty')
    if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
        pass "回复评价成功, id=$ID"
    else
        fail "回复评价失败: $RESP"
    fi
else
    fail "回复评价跳过：REVIEW_ID 为空"
fi

# ===== 7. 举报评价 =====
info "步骤 7/$TOTAL: 举报评价"
if [ -n "$REVIEW_ID" ]; then
    RESP=$(curl -s -X POST "$BASE/api/v1/admin/reviews/$REVIEW_ID/report" \
        -H 'Content-Type: application/json' \
        -d '{"reporterId":"U002","reason":"广告内容"}')
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    ID=$(echo "$RESP" | jq -r '.data.id // empty')
    if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
        REPORT_ID=$ID
        pass "举报评价成功, id=$REPORT_ID"
    else
        fail "举报评价失败: $RESP"
    fi
else
    fail "举报评价跳过：REVIEW_ID 为空"
fi

# ===== 8. 举报列表 =====
info "步骤 8/$TOTAL: 举报列表"
RESP=$(curl -s -X GET "$BASE/api/v1/admin/platform/reviews/reports?status=&page=1&size=20")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 1 ] 2>/dev/null; then
    pass "举报列表查询成功, total=$TOTAL_COUNT"
else
    fail "举报列表查询失败: $RESP"
fi

# ===== 9. 处理举报 =====
info "步骤 9/$TOTAL: 处理举报"
if [ -n "$REPORT_ID" ]; then
    RESP=$(curl -s -X PUT "$BASE/api/v1/admin/platform/reviews/reports/$REPORT_ID/handle" \
        -H 'Content-Type: application/json' \
        -d '{"handleResult":"handled"}')
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

echo "================================================"
echo "测试结果: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT 失败"
[ $FAIL_COUNT -eq 0 ] && exit 0 || exit 1
