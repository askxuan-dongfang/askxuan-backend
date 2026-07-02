#!/bin/bash
# ============================================================
# MVP-2 P4 DIY 全链路闭环测试
# 前置：docker compose up -d && make db-init && make start-all
# 测试链路：创建材料 → 用户登录 → 创建设计(有加持) → 创建DIY订单
#          → 审核通过 → 支付 → mock回调 → 制作完成(有加持)
#          → mock加持完成 → 发货 → 物流签收 → DIY订单完成
# ============================================================

set -o pipefail

BASE=http://localhost:8080
RABBIT_API=http://127.0.0.1:15672
RABBIT_USER=admin
RABBIT_PASS=admin
PASS_COUNT=0
FAIL_COUNT=0
TOTAL=16

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

if ! command -v jq &> /dev/null; then
    echo "错误：需要 jq 工具，请执行 brew install jq"
    exit 1
fi

if ! curl -s --connect-timeout 3 "$BASE/api/v1/auth/admin/login" -o /dev/null; then
    echo "错误：网关 $BASE 不可达，请先执行 make start-all"
    exit 1
fi

info "开始 MVP-2 P4 DIY 全链路闭环测试（共 $TOTAL 步）"
echo "================================================"

# ===== 1. 管理台登录 =====
info "步骤 1/16: 管理台登录"
ADMIN_RESP=$(curl -s -X POST "$BASE/api/v1/auth/admin/login" \
    -H 'Content-Type: application/json' \
    -d '{"account":"admin","password":"123456"}')
ADMIN_TOKEN=$(echo "$ADMIN_RESP" | jq -r '.data.accessToken // empty')
if [ -n "$ADMIN_TOKEN" ]; then
    pass "管理台登录（token=${ADMIN_TOKEN:0:20}...）"
else
    fail "管理台登录失败: $ADMIN_RESP"
fi
[ -z "$ADMIN_TOKEN" ] && echo "无法继续测试，缺少 adminToken" && exit 1
ADMIN_AUTH="Authorization: Bearer $ADMIN_TOKEN"

# ===== 2. 管理台创建材料（使用时间戳避免唯一键冲突） =====
info "步骤 2/16: 管理台创建材料"
MAT_NAME="测试檀木珠$(date +%s)"
MAT_RESP=$(curl -s -X POST "$BASE/api/v1/admin/diy/materials" \
    -H "$ADMIN_AUTH" \
    -H 'Content-Type: application/json' \
    -d "{
        \"name\":\"$MAT_NAME\",
        \"spec\":\"8mm\",
        \"unitPrice\":66.00,
        \"unit\":\"颗\",
        \"category\":\"bead\",
        \"fiveElements\":\"earth\",
        \"image\":\"/assets/test-bead.jpg\",
        \"stock\":200
    }")
MAT_ID=$(echo "$MAT_RESP" | jq -r '.data.id // empty')
if [ -n "$MAT_ID" ]; then
    pass "管理台创建材料（id=$MAT_ID, name=$MAT_NAME）"
else
    fail "管理台创建材料失败: $MAT_RESP"
fi

# ===== 3. 用户登录 =====
info "步骤 3/16: 用户登录"
LOGIN_RESP=$(curl -s -X POST "$BASE/api/v1/auth/login" \
    -H 'Content-Type: application/json' \
    -d '{"phone":"13800138000","code":"1234"}')
TOKEN=$(echo "$LOGIN_RESP" | jq -r '.data.accessToken // empty')
USER_ID=$(echo "$LOGIN_RESP" | jq -r '.data.userInfo.userId // empty')
if [ -n "$TOKEN" ]; then
    pass "用户登录（userId=$USER_ID, token=${TOKEN:0:20}...）"
else
    fail "用户登录失败: $LOGIN_RESP"
fi
[ -z "$TOKEN" ] && echo "无法继续测试，缺少 accessToken" && exit 1
AUTH="Authorization: Bearer $TOKEN"

# ===== 4. 用户创建设计（有加持 E001） =====
info "步骤 4/16: 用户创建设计（blessServiceCode=E001）"
DESIGN_RESP=$(curl -s -X POST "$BASE/api/v1/diy/designs" \
    -H "$AUTH" \
    -H 'Content-Type: application/json' \
    -d "{
        \"userId\":\"$USER_ID\",
        \"name\":\"测试加持手串设计\",
        \"designData\":\"test-design-v1\",
        \"totalPrice\":234.00,
        \"status\":\"draft\",
        \"blessServiceCode\":\"E001\"
    }")
DESIGN_ID=$(echo "$DESIGN_RESP" | jq -r '.data.id // empty')
if [ -n "$DESIGN_ID" ]; then
    pass "用户创建设计（id=$DESIGN_ID, blessServiceCode=E001）"
else
    fail "用户创建设计失败: $DESIGN_RESP"
fi

# ===== 5. 用户创建 DIY 订单 =====
info "步骤 5/16: 用户创建 DIY 订单"
ORDER_RESP=$(curl -s -X POST "$BASE/api/v1/diy/orders" \
    -H "$AUTH" \
    -H 'Content-Type: application/json' \
    -d "{
        \"userId\":\"$USER_ID\",
        \"designId\":${DESIGN_ID:-1},
        \"blessServiceCode\":\"E001\",
        \"addressId\":1,
        \"items\":[{
            \"materialId\":${MAT_ID:-1},
            \"materialName\":\"$MAT_NAME\",
            \"spec\":\"8mm\",
            \"unitPrice\":66.00,
            \"quantity\":1,
            \"subtype\":\"main\"
        }]
    }")
ORDER_ID=$(echo "$ORDER_RESP" | jq -r '.data.id // empty')
ORDER_NO=$(echo "$ORDER_RESP" | jq -r '.data.orderNo // empty')
if [ -n "$ORDER_NO" ]; then
    pass "创建DIY订单（id=$ORDER_ID, orderNo=$ORDER_NO）"
else
    fail "创建DIY订单失败: $ORDER_RESP"
fi

# ===== 6. 管理台审核通过 =====
info "步骤 6/16: 管理台审核通过"
if [ -n "$ORDER_ID" ]; then
    REVIEW_RESP=$(curl -s -X PUT "$BASE/api/v1/admin/diy/orders/$ORDER_ID/review" \
        -H "$ADMIN_AUTH" \
        -H 'Content-Type: application/json' \
        -d '{"action":"approve","reason":""}')
    REVIEW_STATUS=$(echo "$REVIEW_RESP" | jq -r '.data.status // empty')
    if [ "$REVIEW_STATUS" = "in_making" ]; then
        pass "管理台审核通过（status=$REVIEW_STATUS）"
    else
        fail "管理台审核通过失败，期望 in_making 实际 $REVIEW_STATUS: $REVIEW_RESP"
    fi
else
    fail "管理台审核通过（无 orderId，跳过）"
fi

# ===== 7. 创建支付 =====
info "步骤 7/16: 创建支付"
PAY_RESP=$(curl -s -X POST "$BASE/api/v1/payments" \
    -H "$AUTH" \
    -H 'Content-Type: application/json' \
    -d "{
        \"orderType\":\"diy_order\",
        \"orderNo\":\"$ORDER_NO\",
        \"amount\":234.00,
        \"channel\":\"wechat\",
        \"userId\":\"$USER_ID\"
    }")
PAYMENT_NO=$(echo "$PAY_RESP" | jq -r '.data.paymentNo // empty')
if [ -n "$PAYMENT_NO" ]; then
    pass "创建支付（paymentNo=$PAYMENT_NO）"
else
    fail "创建支付失败: $PAY_RESP"
fi

# ===== 8. Mock 微信支付回调 =====
info "步骤 8/16: Mock 微信支付回调"
if [ -n "$PAYMENT_NO" ]; then
    # 用 jq 构建完整的 rawBody JSON，避免 shell 转义问题
    CALLBACK_PAYLOAD=$(jq -n --arg pn "$PAYMENT_NO" '{paymentNo:$pn,tradeNo:"MOCK_DIY_TX_001",result:"success"}')
    CB_BODY=$(jq -n --arg rb "$CALLBACK_PAYLOAD" '{rawBody:$rb}')
    CB_RESP=$(curl -s -X POST "$BASE/api/v1/payments/callback/wechat" \
        -H 'Content-Type: application/json' \
        -d "$CB_BODY")
    CB_CODE=$(echo "$CB_RESP" | jq -r '.data.code // empty')
    if [ "$CB_CODE" = "SUCCESS" ]; then
        pass "Mock 微信支付回调（code=$CB_CODE）"
    else
        fail "Mock 微信支付回调失败: $CB_RESP"
    fi
else
    fail "Mock 微信支付回调（无 paymentNo，跳过）"
fi

# ===== 9. 验证 DIY 订单已进入制作中 =====
info "步骤 9/16: 验证 DIY 订单状态（等待 MQ 消费 3s）"
sleep 3
if [ -n "$ORDER_ID" ]; then
    DETAIL_RESP=$(curl -s "$BASE/api/v1/diy/orders/$ORDER_ID" -H "$AUTH")
    ORDER_STATUS=$(echo "$DETAIL_RESP" | jq -r '.data.status // empty')
    if [ "$ORDER_STATUS" = "in_making" ]; then
        pass "验证 DIY 订单状态（status=$ORDER_STATUS）"
    else
        fail "验证 DIY 订单状态失败，期望 in_making 实际 $ORDER_STATUS: $DETAIL_RESP"
    fi
else
    fail "验证 DIY 订单状态（无 orderId，跳过）"
fi

# ===== 10. 管理台制作完成（有加持） =====
info "步骤 10/16: 管理台制作完成（有加持，创建加持任务）"
if [ -n "$ORDER_ID" ]; then
    MAKE_RESP=$(curl -s -X PUT "$BASE/api/v1/admin/diy/orders/$ORDER_ID/make-complete" \
        -H "$ADMIN_AUTH" \
        -H 'Content-Type: application/json')
    MAKE_STATUS=$(echo "$MAKE_RESP" | jq -r '.data.status // empty')
    TASK_NO=$(echo "$MAKE_RESP" | jq -r '.data.blessingTask.taskNo // empty')
    if [ "$MAKE_STATUS" = "awaiting_blessing" ] && [ -n "$TASK_NO" ]; then
        pass "管理台制作完成（status=$MAKE_STATUS, taskNo=$TASK_NO）"
    else
        fail "管理台制作完成失败，期望 awaiting_blessing 实际 $MAKE_STATUS: $MAKE_RESP"
    fi
else
    fail "管理台制作完成（无 orderId，跳过）"
fi

# ===== 11. Mock 加持完成（通过 RabbitMQ HTTP API 发布 blessing.complete） =====
info "步骤 11/16: Mock 加持完成（RabbitMQ HTTP API 发布 blessing.complete）"
if [ -n "$TASK_NO" ] && [ -n "$ORDER_NO" ]; then
    BLESSING_PAYLOAD=$(jq -n \
        --arg tn "$TASK_NO" \
        --arg dn "$ORDER_NO" \
        --arg t "$(date '+%Y-%m-%d %H:%M:%S')" \
        '{eventType:"blessing.complete",taskNo:$tn,diyOrderId:$dn,templeCode:"T001",masterCode:"M001",status:"completed",time:$t}')
    # 通过 RabbitMQ Management HTTP API 发布消息到 blessing.events exchange
    PUBLISH_RESP=$(curl -s -u "$RABBIT_USER:$RABBIT_PASS" \
        -X POST "$RABBIT_API/api/exchanges/%2f/blessing.events/publish" \
        -H 'Content-Type: application/json' \
        -d "$(jq -n --arg p "$BLESSING_PAYLOAD" '{properties:{content_type:"application/json",delivery_mode:2},routing_key:"",payload:$p,payload_encoding:"string"}')")
    PUBLISH_OK=$(echo "$PUBLISH_RESP" | jq -r '.routed // false')
    if [ "$PUBLISH_OK" = "true" ]; then
        pass "Mock 加持完成（blessing.complete 已发布，taskNo=$TASK_NO）"
    else
        fail "Mock 加持完成失败: $PUBLISH_RESP"
    fi
else
    fail "Mock 加持完成（无 taskNo/orderNo，跳过）"
fi

# ===== 12. 验证 DIY 订单进入待发货 =====
info "步骤 12/16: 验证 DIY 订单状态（等待 MQ 消费 3s）"
sleep 3
if [ -n "$ORDER_ID" ]; then
    DETAIL_RESP=$(curl -s "$BASE/api/v1/diy/orders/$ORDER_ID" -H "$AUTH")
    ORDER_STATUS=$(echo "$DETAIL_RESP" | jq -r '.data.status // empty')
    if [ "$ORDER_STATUS" = "awaiting_shipment" ]; then
        pass "验证 DIY 订单状态（status=$ORDER_STATUS）"
    else
        fail "验证 DIY 订单状态失败，期望 awaiting_shipment 实际 $ORDER_STATUS: $DETAIL_RESP"
    fi
else
    fail "验证 DIY 订单状态（无 orderId，跳过）"
fi

# ===== 13. 管理台发货 =====
info "步骤 13/16: 管理台发货"
if [ -n "$ORDER_ID" ]; then
    SHIP_RESP=$(curl -s -X PUT "$BASE/api/v1/admin/diy/orders/$ORDER_ID/ship" \
        -H "$ADMIN_AUTH" \
        -H 'Content-Type: application/json' \
        -d '{"expressCompany":"SF","trackingNo":"SF-DIY-1234567890"}')
    SHIP_STATUS=$(echo "$SHIP_RESP" | jq -r '.data.status // empty')
    if [ "$SHIP_STATUS" = "shipped" ]; then
        pass "管理台发货（status=$SHIP_STATUS）"
    else
        fail "管理台发货失败，期望 shipped 实际 $SHIP_STATUS: $SHIP_RESP"
    fi
else
    fail "管理台发货（无 orderId，跳过）"
fi

# ===== 14. 验证物流追踪记录创建 =====
info "步骤 14/16: 验证物流追踪记录创建（等待 MQ 消费 3s）"
sleep 3
if [ -n "$ORDER_NO" ]; then
    TRACK_RESP=$(curl -s "$BASE/api/v1/admin/logistics/tracks/$ORDER_NO" \
        -H "$ADMIN_AUTH")
    TRACK_STATUS=$(echo "$TRACK_RESP" | jq -r '.data.status // empty')
    if [ -n "$TRACK_STATUS" ]; then
        pass "验证物流追踪记录已创建（status=$TRACK_STATUS）"
    else
        fail "验证物流追踪记录失败: $TRACK_RESP"
    fi
else
    fail "验证物流追踪记录（无 orderNo，跳过）"
fi

# ===== 15. 物流批量同步（3 轮推进到 signed） =====
info "步骤 15/16: 物流批量同步（3 轮：pending → in_transit → delivered → signed）"
SYNC_OK=true
for i in 1 2 3; do
    SYNC_RESP=$(curl -s -X POST "$BASE/api/v1/admin/logistics/tracks/batch-sync" \
        -H "$ADMIN_AUTH" \
        -H 'Content-Type: application/json' \
        -d '{}')
    SYNC_SUCCESS=$(echo "$SYNC_RESP" | jq -r '.data.success // 0')
    info "  第 $i 轮同步：success=$SYNC_SUCCESS"
    sleep 2
done
# 验证物流已签收
TRACK_RESP=$(curl -s "$BASE/api/v1/admin/logistics/tracks/$ORDER_NO" \
    -H "$ADMIN_AUTH")
TRACK_STATUS=$(echo "$TRACK_RESP" | jq -r '.data.status // empty')
if [ "$TRACK_STATUS" = "signed" ]; then
    pass "物流批量同步完成（track status=$TRACK_STATUS）"
else
    fail "物流批量同步失败，期望 signed 实际 $TRACK_STATUS: $TRACK_RESP"
fi

# ===== 16. 验证 DIY 订单自动完成 =====
info "步骤 16/16: 验证 DIY 订单自动完成（等待 MQ 消费 3s）"
sleep 3
if [ -n "$ORDER_ID" ]; then
    DETAIL_RESP=$(curl -s "$BASE/api/v1/diy/orders/$ORDER_ID" -H "$AUTH")
    ORDER_STATUS=$(echo "$DETAIL_RESP" | jq -r '.data.status // empty')
    if [ "$ORDER_STATUS" = "completed" ]; then
        pass "验证 DIY 订单自动完成（status=$ORDER_STATUS）"
    else
        fail "验证 DIY 订单自动完成失败，期望 completed 实际 $ORDER_STATUS: $DETAIL_RESP"
    fi
else
    fail "验证 DIY 订单自动完成（无 orderId，跳过）"
fi

echo "================================================"
info "测试完成: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT/$TOTAL 失败"

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✓ MVP-2 P4 DIY 全链路闭环测试全部通过！${NC}"
    exit 0
else
    echo -e "${RED}✗ 有 $FAIL_COUNT 步失败，请检查日志 logs/*.log${NC}"
    exit 1
fi
