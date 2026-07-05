#!/bin/bash
# ============================================================
# MVP-2 P3 商城交易全链路闭环测试
# 前置：docker compose up -d && make db-init && make start-all
# 测试链路：创建商品 → 上下架 → 创建订单 → 创建支付 → mock回调
#          → 验证已支付 → 发货 → 验证物流记录 → 确认收货
# ============================================================

set -o pipefail

BASE=http://localhost:8080
PASS_COUNT=0
FAIL_COUNT=0
TOTAL=10

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

info "开始 MVP-2 P3 商城交易闭环测试（共 $TOTAL 步）"
echo "================================================"

# ===== 1. 管理台登录 =====
info "步骤 1/10: 管理台登录"
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
    echo "无法继续测试，缺少 adminToken"
    exit 1
fi

ADMIN_AUTH="Authorization: Bearer $ADMIN_TOKEN"

# ===== 2. 管理台创建商品 =====
info "步骤 2/10: 管理台创建商品"
PROD_RESP=$(curl -s -X POST "$BASE/api/v1/admin/products" \
    -H "$ADMIN_AUTH" \
    -H 'Content-Type: application/json' \
    -d '{
        "name":"测试手串",
        "categoryId":1,
        "description":"MVP-2 闭环测试商品",
        "mainImage":"/assets/test.jpg",
        "price":99.00,
        "marketPrice":128.00,
        "stock":100,
        "tags":"测试",
        "freightTemplateId":0
    }')
PROD_ID=$(echo "$PROD_RESP" | jq -r '.data.id // empty')
if [ -n "$PROD_ID" ]; then
    pass "管理台创建商品（id=$PROD_ID）"
else
    fail "管理台创建商品失败: $PROD_RESP"
fi

# ===== 3. 管理台上架商品 =====
info "步骤 3/10: 管理台上架商品"
if [ -n "$PROD_ID" ]; then
    STATUS_RESP=$(curl -s -X PUT "$BASE/api/v1/admin/products/$PROD_ID/status" \
        -H "$ADMIN_AUTH" \
        -H 'Content-Type: application/json' \
        -d '{"status":"on_shelf"}')
    STATUS_VAL=$(echo "$STATUS_RESP" | jq -r '.data.status // empty')
    if [ "$STATUS_VAL" = "on_shelf" ]; then
        pass "管理台上架商品（status=$STATUS_VAL）"
    else
        fail "管理台上架商品失败: $STATUS_RESP"
    fi
else
    fail "管理台上架商品（无 productId，跳过）"
fi

# ===== 4. 用户登录 =====
info "步骤 4/10: 用户登录"
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

if [ -z "$TOKEN" ]; then
    echo "无法继续测试，缺少 accessToken"
    exit 1
fi

AUTH="Authorization: Bearer $TOKEN"

# ===== 5. 创建订单 =====
info "步骤 5/10: 创建订单"
ORDER_RESP=$(curl -s -X POST "$BASE/api/v1/orders" \
    -H "$AUTH" \
    -H 'Content-Type: application/json' \
    -d "{
        \"userId\":\"$USER_ID\",
        \"addressId\":1,
        \"note\":\"P3 闭环测试订单\",
        \"items\":[{
            \"productId\":${PROD_ID:-1},
            \"skuId\":0,
            \"productName\":\"测试手串\",
            \"price\":99.00,
            \"quantity\":1,
            \"image\":\"/assets/test.jpg\"
        }]
    }")
ORDER_ID=$(echo "$ORDER_RESP" | jq -r '.data.id // empty')
ORDER_NO=$(echo "$ORDER_RESP" | jq -r '.data.orderNo // empty')
if [ -n "$ORDER_NO" ]; then
    pass "创建订单（id=$ORDER_ID, orderNo=$ORDER_NO）"
else
    fail "创建订单失败: $ORDER_RESP"
fi

# ===== 6. 创建支付 =====
info "步骤 6/10: 创建支付"
PAY_RESP=$(curl -s -X POST "$BASE/api/v1/payments" \
    -H "$AUTH" \
    -H 'Content-Type: application/json' \
    -d "{
        \"orderType\":\"shop_order\",
        \"orderNo\":\"$ORDER_NO\",
        \"amount\":99.00,
        \"channel\":\"wechat\",
        \"userId\":\"$USER_ID\"
    }")
PAYMENT_NO=$(echo "$PAY_RESP" | jq -r '.data.paymentNo // empty')
if [ -n "$PAYMENT_NO" ]; then
    pass "创建支付（paymentNo=$PAYMENT_NO）"
else
    fail "创建支付失败: $PAY_RESP"
fi

# ===== 7. Mock 微信支付回调 =====
info "步骤 7/10: Mock 微信支付回调"
if [ -n "$PAYMENT_NO" ]; then
    MOCK_TX="MOCK_TX_$(date +%s)"
    CALLBACK_PAYLOAD=$(jq -n --arg pn "$PAYMENT_NO" --arg tn "$MOCK_TX" '{paymentNo:$pn,tradeNo:$tn,result:"success"}')
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

# ===== 8. 验证订单已支付 =====
info "步骤 8/10: 验证订单已支付（等待 MQ 消费 3s）"
sleep 3
if [ -n "$ORDER_ID" ]; then
    DETAIL_RESP=$(curl -s "$BASE/api/v1/orders/$ORDER_ID" \
        -H "$AUTH")
    ORDER_STATUS=$(echo "$DETAIL_RESP" | jq -r '.data.status // empty')
    if [ "$ORDER_STATUS" = "paid" ]; then
        pass "验证订单已支付（status=$ORDER_STATUS）"
    else
        fail "验证订单已支付失败，期望 paid 实际 $ORDER_STATUS: $DETAIL_RESP"
    fi
else
    fail "验证订单已支付（无 orderId，跳过）"
fi

# ===== 9. 管理台发货 + 验证物流记录 =====
info "步骤 9/10: 管理台发货 + 验证物流记录"
if [ -n "$ORDER_ID" ]; then
    SHIP_RESP=$(curl -s -X PUT "$BASE/api/v1/admin/orders/$ORDER_ID/ship" \
        -H "$ADMIN_AUTH" \
        -H 'Content-Type: application/json' \
        -d '{"expressCompany":"SF","trackingNo":"SF1234567890"}')
    SHIP_STATUS=$(echo "$SHIP_RESP" | jq -r '.data.status // empty')
    if [ "$SHIP_STATUS" = "shipped" ]; then
        pass "管理台发货（status=$SHIP_STATUS）"
        # 等待 logistics-service 消费 order.events 创建物流追踪记录
        sleep 3
        TRACK_RESP=$(curl -s "$BASE/api/v1/admin/logistics/tracks/$ORDER_NO" \
            -H "$ADMIN_AUTH")
        TRACK_STATUS=$(echo "$TRACK_RESP" | jq -r '.data.status // empty')
        if [ -n "$TRACK_STATUS" ]; then
            pass "验证物流记录已创建（status=$TRACK_STATUS）"
        else
            fail "验证物流记录失败: $TRACK_RESP"
        fi
    else
        fail "管理台发货失败: $SHIP_RESP"
        fail "验证物流记录（跳过）"
    fi
else
    fail "管理台发货（无 orderId，跳过）"
    fail "验证物流记录（跳过）"
fi

# ===== 10. 确认收货 =====
info "步骤 10/10: 确认收货"
if [ -n "$ORDER_ID" ]; then
    CONFIRM_RESP=$(curl -s -X PUT "$BASE/api/v1/orders/$ORDER_ID/confirm" \
        -H "$AUTH")
    CONFIRM_STATUS=$(echo "$CONFIRM_RESP" | jq -r '.data.status // empty')
    if [ "$CONFIRM_STATUS" = "completed" ]; then
        pass "确认收货（status=$CONFIRM_STATUS）"
    else
        fail "确认收货失败，期望 completed 实际 $CONFIRM_STATUS: $CONFIRM_RESP"
    fi
else
    fail "确认收货（无 orderId，跳过）"
fi

echo "================================================"
info "测试完成: $PASS_COUNT/$TOTAL 通过, $FAIL_COUNT/$TOTAL 失败"

if [ "$FAIL_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✓ MVP-2 P3 商城交易全链路闭环测试全部通过！${NC}"
    exit 0
else
    echo -e "${RED}✗ 有 $FAIL_COUNT 步失败，请检查日志 logs/*.log${NC}"
    exit 1
fi
