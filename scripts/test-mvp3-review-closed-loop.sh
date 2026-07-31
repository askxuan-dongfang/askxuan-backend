#!/bin/bash
# MVP-3 预约评价闭环：预约支付 → 确认 → 开始 → 完成 → 评价 → 回复 → 举报处理

set -o pipefail

BASE="${BASE_URL:-http://localhost:8080}"
PASS_COUNT=0
FAIL_COUNT=0
TOTAL=15

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

check_response() {
    local label="$1"
    local response="$2"
    local expression="$3"
    if echo "$response" | jq -e "$expression" >/dev/null 2>&1; then
        pass "$label"
        return 0
    fi
    fail "$label: $response"
    return 1
}

if ! command -v jq >/dev/null 2>&1; then
    echo "错误：需要 jq 工具"
    exit 1
fi

info "开始 MVP-3 预约评价闭环测试（共 $TOTAL 步）"
echo "================================================"

# 1. 用户登录
RESP=$(curl -sS -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d '{"phone":"13800138000","password":"123456"}')
USER_TOKEN=$(echo "$RESP" | jq -r '.data.accessToken // empty')
check_response "用户登录" "$RESP" '.code == 0 and (.data.accessToken | length > 0)'
USER_AUTH="Authorization: Bearer $USER_TOKEN"

# 2. 平台管理员登录（预约确认、评价回复、举报处理）
RESP=$(curl -sS -X POST "$BASE/api/v1/auth/admin/login" \
    -H 'Content-Type: application/json' \
    -d '{"account":"admin","password":"123456"}')
ADMIN_TOKEN=$(echo "$RESP" | jq -r '.data.accessToken // empty')
check_response "平台管理员登录" "$RESP" '.code == 0 and (.data.accessToken | length > 0)'
ADMIN_AUTH="Authorization: Bearer $ADMIN_TOKEN"

# 3. 法师登录（开始、完成本人预约）
RESP=$(curl -sS -X POST "$BASE/api/v1/auth/admin/login" \
    -H 'Content-Type: application/json' \
    -d '{"account":"zhihai","password":"123456"}')
MASTER_TOKEN=$(echo "$RESP" | jq -r '.data.accessToken // empty')
check_response "法师登录" "$RESP" '.code == 0 and (.data.accessToken | length > 0)'
MASTER_AUTH="Authorization: Bearer $MASTER_TOKEN"

if date -v+45d +%F >/dev/null 2>&1; then
    BOOKING_DATE=$(date -v+45d +%F)
else
    BOOKING_DATE=$(date -d '+45 days' +%F)
fi

# 4. 查询权威服务价格和可用时段
RESP=$(curl -sS "$BASE/api/v1/bookings/availability?templeId=T001&serviceId=S001&date=$BOOKING_DATE")
SLOT_CODE=$(echo "$RESP" | jq -r '[.data.slots[] | select(.available == true) | .slotCode][0] // empty')
check_response "查询预约时段" "$RESP" '.code == 0 and (.data.serviceFee >= 0) and ([.data.slots[] | select(.available == true)] | length > 0)'

# 5. 创建预约并完成本地 mock 支付
REQUEST_ID="review-loop-$(date +%s)-$RANDOM"
RESP=$(curl -sS -X POST "$BASE/api/v1/bookings" \
    -H "$USER_AUTH" \
    -H 'Content-Type: application/json' \
    -d "{
        \"requestId\":\"$REQUEST_ID\",
        \"templeId\":\"T001\",
        \"masterId\":\"M001\",
        \"serviceId\":\"S001\",
        \"bookingDate\":\"$BOOKING_DATE\",
        \"slotCode\":\"$SLOT_CODE\",
        \"meritMoney\":10,
        \"meritMoneyTier\":\"随喜\"
    }")
BOOKING_ID=$(echo "$RESP" | jq -r '.data.id // empty')
check_response "创建并模拟支付预约" "$RESP" '.code == 0 and .data.status == "pending" and .data.paymentStatus == "success" and .data.simulated == true'

# 6. 寺院侧确认预约
RESP=$(curl -sS -X PUT "$BASE/api/v1/admin/bookings/$BOOKING_ID/confirm" \
    -H "$ADMIN_AUTH" -H 'Content-Type: application/json' \
    -d '{"remark":"评价闭环确认"}')
check_response "确认预约" "$RESP" '.code == 0 and .data.status == "confirmed"'

# 7. 法师开始服务
RESP=$(curl -sS -X PUT "$BASE/api/v1/admin/masters/bookings/$BOOKING_ID/start" \
    -H "$MASTER_AUTH" -H 'Content-Type: application/json' \
    -d '{"remark":"开始服务"}')
check_response "法师开始服务" "$RESP" '.code == 0 and .data.status == "in_progress"'

# 8. 法师完成服务
RESP=$(curl -sS -X PUT "$BASE/api/v1/admin/masters/bookings/$BOOKING_ID/complete" \
    -H "$MASTER_AUTH" -H 'Content-Type: application/json' \
    -d '{"remark":"服务完成"}')
check_response "法师完成服务" "$RESP" '.code == 0 and .data.status == "completed"'

# 9. 预约本人提交评价
RESP=$(curl -sS -X POST "$BASE/api/v1/bookings/$BOOKING_ID/review" \
    -H "$USER_AUTH" -H 'Content-Type: application/json' \
    -d '{"rating":5,"content":"预约流程清晰，服务认真。","images":[]}')
BOOKING_REVIEW_ID=$(echo "$RESP" | jq -r '.data.reviewId // empty')
check_response "本人提交预约评价" "$RESP" '.code == 0 and .data.reviewId > 0'

# 10. C 端读取预约评价
RESP=$(curl -sS "$BASE/api/v1/bookings/$BOOKING_ID/review" -H "$USER_AUTH")
check_response "C端读取预约评价" "$RESP" '.code == 0 and .data.rating == 5 and .data.bookingId == "'"$BOOKING_ID"'"'

# booking-service 通过 MQ 同步 review-service 读模型。
REVIEW_ID=""
for _ in $(seq 1 10); do
    RESP=$(curl -sS "$BASE/api/v1/reviews?targetType=booking&targetId=$BOOKING_ID&page=1&size=20" -H "$USER_AUTH")
    REVIEW_ID=$(echo "$RESP" | jq -r '.data.list[0].id // empty')
    [ -n "$REVIEW_ID" ] && break
    sleep 1
done

# 11. 公共评价读模型
check_response "评价读模型已同步" "$RESP" '.code == 0 and .data.total == 1 and .data.list[0].rating == 5'

# 12. 管理台读取评价详情
RESP=$(curl -sS "$BASE/api/v1/admin/reviews/$REVIEW_ID" -H "$ADMIN_AUTH")
check_response "管理台读取评价详情" "$RESP" '.code == 0 and .data.targetId == "'"$BOOKING_ID"'"'

# 13. 管理台回复评价
RESP=$(curl -sS -X POST "$BASE/api/v1/admin/reviews/$REVIEW_ID/reply" \
    -H "$ADMIN_AUTH" -H 'Content-Type: application/json' \
    -d '{"replierType":"platform","replierId":"admin","content":"感谢您的真实评价。"}')
check_response "管理台回复评价" "$RESP" '.code == 0 and .data.id > 0'

# 14. 举报评价并进入待处理列表
RESP=$(curl -sS -X POST "$BASE/api/v1/admin/reviews/$REVIEW_ID/report" \
    -H "$ADMIN_AUTH" -H 'Content-Type: application/json' \
    -d '{"reporterId":"admin","reason":"闭环测试举报"}')
REPORT_ID=$(echo "$RESP" | jq -r '.data.id // empty')
if check_response "创建评价举报" "$RESP" '.code == 0 and .data.id > 0'; then
    LIST_RESP=$(curl -sS "$BASE/api/v1/admin/platform/reviews/reports?status=pending&page=1&size=100" -H "$ADMIN_AUTH")
    if ! echo "$LIST_RESP" | jq -e '.code == 0 and ([.data.list[].id] | index('"$REPORT_ID"')) != null' >/dev/null 2>&1; then
        fail "举报未进入待处理列表: $LIST_RESP"
    fi
fi

# 15. 平台处理举报
RESP=$(curl -sS -X PUT "$BASE/api/v1/admin/platform/reviews/reports/$REPORT_ID/handle" \
    -H "$ADMIN_AUTH" -H 'Content-Type: application/json' \
    -d '{"handleResult":"handled","remark":"闭环测试处理"}')
check_response "平台处理举报" "$RESP" '.code == 0 and .data.status == "handled"'

echo "================================================"
echo "测试结果: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT 失败"
[ "$FAIL_COUNT" -eq 0 ] && exit 0 || exit 1
