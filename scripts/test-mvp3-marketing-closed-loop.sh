#!/bin/bash
# ============================================================
# MVP-3 营销服务闭环测试
# 前置：启动 marketing-service（端口 8096）
# ============================================================

set -o pipefail

BASE=http://localhost:8096
PASS_COUNT=0
FAIL_COUNT=0
TOTAL=11

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

info "开始 MVP-3 营销闭环测试（共 $TOTAL 步）"
echo "================================================"

BANNER_ID=""
COUPON_ID=""

# ===== 1. C端 Banner =====
info "步骤 1/$TOTAL: C端 Banner 列表"
RESP=$(curl -s -X GET "$BASE/api/v1/marketing/banners")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 0 ] 2>/dev/null; then
    pass "C端 Banner 查询成功, total=$TOTAL_COUNT"
else
    fail "C端 Banner 查询失败: $RESP"
fi

# ===== 2. 创建 Banner =====
info "步骤 2/$TOTAL: 创建 Banner"
RESP=$(curl -s -X POST "$BASE/api/v1/admin/marketing/banners" \
    -H 'Content-Type: application/json' \
    -d '{"title":"测试Banner","imageUrl":"http://example.com/img.jpg","linkType":"temple","linkValue":"T001","sort":1,"startTime":"2026-01-01 00:00:00","endTime":"2026-12-31 23:59:59"}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
ID=$(echo "$RESP" | jq -r '.data.id // empty')
if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
    BANNER_ID=$ID
    pass "创建 Banner 成功, id=$BANNER_ID"
else
    fail "创建 Banner 失败: $RESP"
fi

# ===== 3. 更新 Banner =====
info "步骤 3/$TOTAL: 更新 Banner"
if [ -n "$BANNER_ID" ]; then
    RESP=$(curl -s -X PUT "$BASE/api/v1/admin/marketing/banners/$BANNER_ID" \
        -H 'Content-Type: application/json' \
        -d '{"title":"更新Banner","status":"enabled"}')
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    ID=$(echo "$RESP" | jq -r '.data.id // empty')
    if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
        pass "更新 Banner 成功, id=$ID"
    else
        fail "更新 Banner 失败: $RESP"
    fi
else
    fail "更新 Banner 跳过：BANNER_ID 为空"
fi

# ===== 4. C端活动 =====
info "步骤 4/$TOTAL: C端活动列表"
RESP=$(curl -s -X GET "$BASE/api/v1/marketing/activities")
CODE=$(echo "$RESP" | jq -r '.code // 0')
if [ "$CODE" = "0" ]; then
    pass "C端活动查询成功"
else
    fail "C端活动查询失败: $RESP"
fi

# ===== 5. 创建活动 =====
info "步骤 5/$TOTAL: 创建活动"
RESP=$(curl -s -X POST "$BASE/api/v1/admin/marketing/activities" \
    -H 'Content-Type: application/json' \
    -d '{"name":"测试活动","type":"festival","startTime":"2026-01-01 00:00:00","endTime":"2026-12-31 23:59:59","config":"{}"}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
ID=$(echo "$RESP" | jq -r '.data.id // empty')
if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
    pass "创建活动成功, id=$ID"
else
    fail "创建活动失败: $RESP"
fi

# ===== 6. C端优惠券 =====
info "步骤 6/$TOTAL: C端优惠券列表"
RESP=$(curl -s -X GET "$BASE/api/v1/marketing/coupons")
CODE=$(echo "$RESP" | jq -r '.code // 0')
if [ "$CODE" = "0" ]; then
    pass "C端优惠券查询成功"
else
    fail "C端优惠券查询失败: $RESP"
fi

# ===== 7. 创建优惠券 =====
info "步骤 7/$TOTAL: 创建优惠券"
RESP=$(curl -s -X POST "$BASE/api/v1/admin/marketing/coupons" \
    -H 'Content-Type: application/json' \
    -d '{"name":"测试券","type":"full_reduce","value":10,"minAmount":100,"startTime":"2026-01-01 00:00:00","endTime":"2026-12-31 23:59:59","totalCount":100}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
ID=$(echo "$RESP" | jq -r '.data.id // empty')
if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
    COUPON_ID=$ID
    pass "创建优惠券成功, id=$COUPON_ID"
else
    fail "创建优惠券失败: $RESP"
fi

# ===== 8. 领取优惠券 =====
info "步骤 8/$TOTAL: 领取优惠券"
if [ -n "$COUPON_ID" ]; then
    RESP=$(curl -s -X POST "$BASE/api/v1/marketing/coupons/$COUPON_ID/receive?userId=U001")
    CODE=$(echo "$RESP" | jq -r '.code // 0')
    ID=$(echo "$RESP" | jq -r '.data.id // empty')
    NAME=$(echo "$RESP" | jq -r '.data.name // empty')
    if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null && [ -n "$NAME" ] && [ "$NAME" != "null" ]; then
        pass "领取优惠券成功, id=$ID, name=$NAME"
    else
        fail "领取优惠券失败: $RESP"
    fi
else
    fail "领取优惠券跳过：COUPON_ID 为空"
fi

# ===== 9. 我的优惠券 =====
info "步骤 9/$TOTAL: 我的优惠券"
RESP=$(curl -s -X GET "$BASE/api/v1/marketing/my-coupons?userId=U001")
CODE=$(echo "$RESP" | jq -r '.code // 0')
TOTAL_COUNT=$(echo "$RESP" | jq -r '.data.total // 0')
if [ "$CODE" = "0" ] && [ "$TOTAL_COUNT" -ge 1 ] 2>/dev/null; then
    pass "我的优惠券查询成功, total=$TOTAL_COUNT"
else
    fail "我的优惠券查询失败: $RESP"
fi

# ===== 10. C端推荐位 =====
info "步骤 10/$TOTAL: C端推荐位列表"
RESP=$(curl -s -X GET "$BASE/api/v1/marketing/recommends?type=temple")
CODE=$(echo "$RESP" | jq -r '.code // 0')
if [ "$CODE" = "0" ]; then
    pass "C端推荐位查询成功"
else
    fail "C端推荐位查询失败: $RESP"
fi

# ===== 11. 更新推荐位 =====
info "步骤 11/$TOTAL: 更新推荐位"
RESP=$(curl -s -X PUT "$BASE/api/v1/admin/marketing/recommends/1" \
    -H 'Content-Type: application/json' \
    -d '{"sort":99,"status":"enabled"}')
CODE=$(echo "$RESP" | jq -r '.code // 0')
ID=$(echo "$RESP" | jq -r '.data.id // empty')
if [ "$CODE" = "0" ] && [ -n "$ID" ] && [ "$ID" != "null" ] && [ "$ID" -gt 0 ] 2>/dev/null; then
    pass "更新推荐位成功, id=$ID"
else
    fail "更新推荐位失败: $RESP"
fi

echo "================================================"
echo "测试结果: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT 失败"
[ $FAIL_COUNT -eq 0 ] && exit 0 || exit 1
