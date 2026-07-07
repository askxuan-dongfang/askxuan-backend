#!/bin/bash
# ============================================================
# MVP-1 预约祈福全链路闭环测试
# 前置：docker compose up -d && make db-init && make start-all
# ============================================================

set -eo pipefail

BASE=http://localhost:8080
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

# 检查网关是否可用
if ! curl -s --connect-timeout 3 "$BASE/api/v1/auth/admin/login" -o /dev/null; then
    echo "错误：网关 $BASE 不可达，请先执行 make start-all"
    exit 1
fi

info "开始 MVP-1 闭环测试（共 $TOTAL 步）"
echo "================================================"

# ===== 1. 用户注册 =====
info "步骤 1/10: 用户注册"
USER_ID=""
MOBILE="13900000$((RANDOM % 10000))"
REG_RESP=$(curl -s -X POST "$BASE/api/v1/users/register" \
    -H 'Content-Type: application/json' \
    -d "{\"mobile\":\"$MOBILE\",\"code\":\"1234\",\"nickname\":\"测试用户\"}")
REG_CODE=$(echo "$REG_RESP" | jq -r '.code // 0')
if [ "$REG_CODE" = "0" ] || echo "$REG_RESP" | jq -e '.data.userId' > /dev/null 2>&1; then
    USER_ID=$(echo "$REG_RESP" | jq -r '.data.userId // empty')
    if [ -z "$USER_ID" ]; then
        # 可能用户已存在，继续登录
        info "注册返回: $REG_RESP（可能已注册，继续登录）"
        pass "用户注册（userId=已有用户）"
    else
        pass "用户注册（userId=$USER_ID）"
    fi
else
    fail "用户注册失败: $REG_RESP"
    USER_ID=""
fi

# ===== 2. 用户登录 =====
info "步骤 2/10: 用户登录"
LOGIN_RESP=$(curl -s -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d "{\"phone\":\"$MOBILE\",\"code\":\"1234\"}")
TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.accessToken // empty')
if [ -n "$TOKEN" ]; then
    # 如果注册没拿到 userId，从登录响应取
    if [ -z "$USER_ID" ]; then
        USER_ID=$(echo "$LOGIN_RESP" | jq -r '.data.userInfo.userId // empty')
    fi
    pass "用户登录（token=${TOKEN:0:20}...）"
else
    fail "用户登录失败: $LOGIN_RESP"
fi

if [ -z "$TOKEN" ]; then
    echo "无法继续测试，缺少 accessToken"
    exit 1
fi

AUTH_HEADER="Authorization: Bearer $TOKEN"

# ===== 3. 浏览寺院列表 =====
info "步骤 3/10: 浏览寺院列表"
TEMPLE_RESP=$(curl -s "$BASE/api/v1/temples?page=1&size=10" \
    -H "$AUTH_HEADER")
TEMPLE_COUNT=$(echo "$TEMPLE_RESP" | jq -r '.data.total // .data.list | length // 0')
if [ "$TEMPLE_COUNT" -gt 0 ] 2>/dev/null; then
    pass "浏览寺院列表（共 $TEMPLE_COUNT 座寺院）"
else
    fail "浏览寺院列表失败: $TEMPLE_RESP"
fi

# ===== 4. 浏览法师列表 =====
info "步骤 4/10: 浏览法师列表"
MASTER_RESP=$(curl -s "$BASE/api/v1/masters?page=1&size=10" \
    -H "$AUTH_HEADER")
MASTER_COUNT=$(echo "$MASTER_RESP" | jq -r '.data.total // .data.list | length // 0')
if [ "$MASTER_COUNT" -gt 0 ] 2>/dev/null; then
    pass "浏览法师列表（共 $MASTER_COUNT 位法师）"
else
    fail "浏览法师列表失败: $MASTER_RESP"
fi

# ===== 5. 创建预约 =====
info "步骤 5/10: 创建预约"
BOOKING_RESP=$(curl -s -X POST "$BASE/api/v1/bookings" \
    -H "$AUTH_HEADER" \
    -H 'Content-Type: application/json' \
    -d "{
        \"userId\":\"$USER_ID\",
        \"templeId\":\"T001\",
        \"masterId\":\"M001\",
        \"serviceId\":\"S001\",
        \"bookingDate\":\"2026-08-15\",
        \"timeSlot\":\"09:00-10:00\",
        \"meritMoney\":200,
        \"meritMoneyTier\":\"中额\"
    }")
BOOKING_ID=$(echo "$BOOKING_RESP" | jq -r '.data.id // empty')
BOOKING_STATUS=$(echo "$BOOKING_RESP" | jq -r '.data.status // empty')
if [ -n "$BOOKING_ID" ]; then
    pass "创建预约（id=$BOOKING_ID, status=$BOOKING_STATUS）"
else
    fail "创建预约失败: $BOOKING_RESP"
fi

# ===== 6. 查询站内消息（等待 MQ 消费）=====
info "步骤 6/10: 查询站内消息（等待 MQ 消费 2s）"
sleep 2
MSG_RESP=$(curl -s "$BASE/api/v1/messages/list?userId=$USER_ID&page=1&size=20" \
    -H "$AUTH_HEADER")
MSG_COUNT=$(echo "$MSG_RESP" | jq -r '.data.total // .total // 0')
if [ "$MSG_COUNT" -gt 0 ] 2>/dev/null; then
    pass "查询站内消息（共 $MSG_COUNT 条消息）"
else
    fail "查询站内消息失败或无消息: $MSG_RESP"
fi

# ===== 7. 管理台登录 =====
info "步骤 7/10: 管理台登录"
ADMIN_RESP=$(curl -s -X POST "$BASE/api/v1/auth/admin/login" \
    -H 'Content-Type: application/json' \
    -d '{"account":"admin","password":"123456"}')
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.data.accessToken // empty')
if [ -n "$ADMIN_TOKEN" ]; then
    pass "管理台登录（token=${ADMIN_TOKEN:0:20}...）"
else
    fail "管理台登录失败: $ADMIN_RESP"
fi

if [ -z "$ADMIN_TOKEN" ]; then
    echo "无法继续管理台测试，缺少 adminToken"
    # 跳过后续步骤
    fail "管理台查询预约列表（跳过）"
    fail "管理台确认预约（跳过）"
    fail "查询确认通知消息（跳过）"
    echo "================================================"
    info "测试完成: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT/$TOTAL 失败"
    exit 1
fi

ADMIN_AUTH_HEADER="Authorization: Bearer $ADMIN_TOKEN"

# ===== 8. 管理台查询预约列表 =====
info "步骤 8/10: 管理台查询预约列表"
ADMIN_BOOKING_RESP=$(curl -s "$BASE/api/v1/admin/bookings?templeId=T001&page=1&size=10" \
    -H "$ADMIN_AUTH_HEADER")
ADMIN_BOOKING_COUNT=$(echo "$ADMIN_BOOKING_RESP" | jq -r '.data.total // .data.list | length // 0')
if [ "$ADMIN_BOOKING_COUNT" -gt 0 ] 2>/dev/null; then
    pass "管理台查询预约列表（共 $ADMIN_BOOKING_COUNT 条预约）"
else
    fail "管理台查询预约列表失败: $ADMIN_BOOKING_RESP"
fi

# ===== 9. 管理台确认预约 =====
info "步骤 9/10: 管理台确认预约"
if [ -n "$BOOKING_ID" ]; then
    CONFIRM_RESP=$(curl -s -X PUT "$BASE/api/v1/admin/bookings/$BOOKING_ID/confirm" \
        -H "$ADMIN_AUTH_HEADER" \
        -H 'Content-Type: application/json' \
        -d '{"remark":"测试确认"}')
    CONFIRM_STATUS=$(echo "$CONFIRM_RESP" | jq -r '.data.status // empty')
    if [ "$CONFIRM_STATUS" = "confirmed" ]; then
        pass "管理台确认预约（status=$CONFIRM_STATUS）"
    else
        fail "管理台确认预约失败: $CONFIRM_RESP"
    fi
else
    fail "管理台确认预约（无 bookingId，跳过）"
fi

# ===== 10. 查询消息（应收到确认通知）=====
info "步骤 10/10: 查询确认通知消息（等待 MQ 消费 2s）"
sleep 2
MSG_RESP2=$(curl -s "$BASE/api/v1/messages/list?userId=$USER_ID&page=1&size=20" \
    -H "$AUTH_HEADER")
MSG_COUNT2=$(echo "$MSG_RESP2" | jq -r '.data.total // .total // 0')
if [ "$MSG_COUNT2" -gt 0 ] 2>/dev/null; then
    pass "查询确认通知消息（共 $MSG_COUNT2 条消息）"
else
    fail "查询确认通知消息失败或无消息: $MSG_RESP2"
fi

echo "================================================"
info "测试完成: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT/$TOTAL 失败"

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✓ MVP-1 预约祈福全链路闭环测试全部通过！${NC}"
    exit 0
else
    echo -e "${RED}✗ 有 $FAIL_COUNT 步失败，请检查日志 logs/*.log${NC}"
    exit 1
fi
