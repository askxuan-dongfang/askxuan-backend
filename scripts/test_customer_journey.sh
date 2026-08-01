#!/bin/bash
# C 端用户全流程闭环测试：注册→登录→浏览→预约→下单→聊天
GATEWAY="http://127.0.0.1:8080"
PASS=0; FAIL=0
RESULTS=()

json_code() { python3 -c "import sys,json;print(json.load(sys.stdin).get('code',''))" 2>/dev/null; }
json_data() { python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('$1',''))" 2>/dev/null; }

check() {
  local name="$1" expected="$2" actual="$3" detail="$4"
  if [ "$actual" = "$expected" ]; then
    PASS=$((PASS+1)); RESULTS+=("PASS $name")
    printf "  ✓ %s\n" "$name"
  else
    FAIL=$((FAIL+1)); RESULTS+=("FAIL $name [期望:$expected 实际:$actual] $detail")
    printf "  ✗ %s [期望:%s 实际:%s] %s\n" "$name" "$expected" "$actual" "$detail"
  fi
}

# 生成唯一手机号和昵称（时间戳）
# 手机号格式：139 + 8 位时间戳 = 11 位
PHONE="139$(date +%y%m%d%H%M | tail -c 9)"
# 兜底保证 11 位
PHONE=$(printf "139%08d" "$(( $(date +%s) % 100000000 ))")
NICKNAME="闭环用户_$(date +%s)"
echo "=========================================="
echo "  C 端用户全流程闭环测试"
echo "  测试手机号: $PHONE"
echo "=========================================="

# ===== 1. 注册 =====
echo ""
echo "--- 1. 用户注册 ---"
# 注：auth-service/user-service MVP-1 阶段验证码固定 1234
R=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/users/register" \
  -H "Content-Type: application/json" \
  -d "{\"mobile\":\"$PHONE\",\"code\":\"1234\",\"nickname\":\"$NICKNAME\"}")
check "1.1 用户注册" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 200)"

# ===== 2. 登录 =====
echo ""
echo "--- 2. 用户登录 ---"
R=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"phone\":\"$PHONE\",\"code\":\"1234\"}")
check "2.1 手机号登录" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 200)"
TOKEN=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('accessToken',''))" 2>/dev/null)
USER_ID=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('userInfo',{}).get('userId',''))" 2>/dev/null)
echo "  → userId=$USER_ID"
check "2.2 获取到 accessToken" "non_empty" "${TOKEN:+non_empty}" ""

if [ -z "$TOKEN" ]; then
  echo "!!! 登录失败，无法继续后续测试"
  exit 1
fi

AUTH="Authorization: Bearer $TOKEN"

# ===== 3. 浏览（公开接口）=====
echo ""
echo "--- 3. 浏览（寺院/法师/商品）---"
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/temples?page=1&size=10")
check "3.1 寺院列表" "0" "$(echo "$R" | json_code)" "total=$(echo "$R" | json_data total)"
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/temples/T001")
check "3.2 寺院详情" "0" "$(echo "$R" | json_code)" ""
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/temples/T001/services")
check "3.3 寺院服务" "0" "$(echo "$R" | json_code)" ""
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/masters?page=1&size=10")
check "3.4 法师列表" "0" "$(echo "$R" | json_code)" "total=$(echo "$R" | json_data total)"
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/masters/M001")
check "3.5 法师详情" "0" "$(echo "$R" | json_code)" ""
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/products?page=1&size=10")
check "3.6 商品列表" "0" "$(echo "$R" | json_code)" ""
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/products/categories")
check "3.7 商品分类" "0" "$(echo "$R" | json_code)" ""
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/marketing/banners")
check "3.8 首页 Banner" "0" "$(echo "$R" | json_code)" ""

# ===== 4. 用户信息 =====
echo ""
echo "--- 4. 用户信息 ---"
R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/users/profile")
check "4.1 获取用户信息" "0" "$(echo "$R" | json_code)" ""
R=$(curl -s --max-time 10 -X PUT -H "$AUTH" -H "Content-Type: application/json" \
  "$GATEWAY/api/v1/users/profile" \
  -d "{\"nickname\":\"$NICKNAME 已更新\",\"avatar\":\"https://cdn.test/avatar.png\"}")
check "4.2 更新用户信息" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 200)"

# ===== 5. 预约闭环 =====
echo ""
echo "--- 5. 预约闭环 ---"
BOOKING_DATE=$(date -v+1d +%Y-%m-%d 2>/dev/null || date -d "+1 day" +%Y-%m-%d)
R=$(curl -s --max-time 10 \
  "$GATEWAY/api/v1/bookings/availability?templeId=T001&serviceId=S001&date=$BOOKING_DATE")
SLOT_CODE=$(echo "$R" | python3 -c "import sys,json; slots=json.load(sys.stdin).get('data',{}).get('slots',[]); print(next((s.get('slotCode','') for s in slots if s.get('available')), ''))" 2>/dev/null)
R=$(curl -s --max-time 10 -X POST -H "$AUTH" -H "Content-Type: application/json" \
  "$GATEWAY/api/v1/bookings" \
  -d "{\"requestId\":\"journey-booking-$(date +%s)-$RANDOM\",\"userId\":\"$USER_ID\",\"templeId\":\"T001\",\"masterId\":\"M001\",\"serviceId\":\"S001\",\"bookingDate\":\"$BOOKING_DATE\",\"slotCode\":\"$SLOT_CODE\",\"meritMoney\":99,\"meritMoneyTier\":\"基础\",\"note\":\"自动化测试预约\"}")
check "5.1 创建预约" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"
BK_NO=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('id','') or d.get('bookingNo',''))" 2>/dev/null)
echo "  → bookingNo=$BK_NO"

if [ -n "$BK_NO" ]; then
  R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/bookings/$BK_NO")
  check "5.2 查询预约详情" "0" "$(echo "$R" | json_code)" ""
fi

R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/bookings?userId=$USER_ID&page=1&size=20")
check "5.3 查询我的预约列表" "0" "$(echo "$R" | json_code)" "total=$(echo "$R" | json_data total)"

# ===== 6. DIY 设计与下单闭环 =====
echo ""
echo "--- 6. DIY 设计与下单闭环 ---"
# 6.1 查询材料库（公开接口，无需 JWT）
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/diy/materials?page=1&size=10")
check "6.1 DIY 材料库" "0" "$(echo "$R" | json_code)" ""
# 6.2 保存设计
R=$(curl -s --max-time 10 -X POST -H "$AUTH" -H "Content-Type: application/json" \
  "$GATEWAY/api/v1/diy/designs" \
  -d "{\"userId\":\"$USER_ID\",\"name\":\"我的护身符\",\"designData\":\"{\\\"material\\\":\\\"gold\\\"}\",\"totalPrice\":199,\"status\":\"draft\"}")
check "6.2 保存 DIY 设计" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"
DESIGN_ID=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('id','') or d.get('designId',''))" 2>/dev/null)
echo "  → designId=$DESIGN_ID"

R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/diy/designs?userId=$USER_ID&page=1&size=10")
check "6.3 我的 DIY 设计列表" "0" "$(echo "$R" | json_code)" ""

# 6.4 创建 DIY 订单（DiyOrderItem 需 materialId+materialName+spec+unitPrice+quantity）
if [ -n "$DESIGN_ID" ]; then
  R=$(curl -s --max-time 10 -X POST -H "$AUTH" -H "Content-Type: application/json" \
    "$GATEWAY/api/v1/diy/orders" \
    -d "{\"userId\":\"$USER_ID\",\"designId\":$DESIGN_ID,\"items\":[{\"materialId\":1,\"materialName\":\"小叶紫檀圆珠\",\"spec\":\"10mm\",\"unitPrice\":199,\"quantity\":1}],\"addressId\":1}")
  check "6.4 创建 DIY 订单" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"
fi

# ===== 7. 商城订单闭环 =====
echo ""
echo "--- 7. 商城订单闭环 ---"
# 先查询商品列表拿一个 productId
PRODUCT_ID=$(curl -s --max-time 10 "$GATEWAY/api/v1/products?page=1&size=10" | \
  python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});l=d.get('list',[]);print(l[0].get('id','') if l else '')" 2>/dev/null)

# 7.1 创建地址
R=$(curl -s --max-time 10 -X POST -H "$AUTH" -H "Content-Type: application/json" \
  "$GATEWAY/api/v1/users/addresses" \
  -d "{\"name\":\"测试用户\",\"phone\":\"$PHONE\",\"province\":\"浙江省\",\"city\":\"杭州市\",\"district\":\"西湖区\",\"detail\":\"灵隐路1号\"}")
check "7.1 新增收货地址" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"
ADDR_ID=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('id','') or d.get('addressId',''))" 2>/dev/null)
echo "  → addressId=$ADDR_ID"

R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/users/addresses")
check "7.2 地址列表" "0" "$(echo "$R" | json_code)" ""

# 7.2 创建商城订单（ShopOrderItem 需 productId+productName+price+quantity）
if [ -n "$PRODUCT_ID" ] && [ -n "$ADDR_ID" ]; then
  R=$(curl -s --max-time 10 -X POST -H "$AUTH" -H "Content-Type: application/json" \
    "$GATEWAY/api/v1/orders" \
    -d "{\"requestId\":\"journey-order-$(date +%s)-$RANDOM\",\"userId\":\"$USER_ID\",\"addressId\":$ADDR_ID,\"note\":\"自动化测试订单\",\"items\":[{\"productId\":$PRODUCT_ID,\"skuId\":0,\"productName\":\"测试商品\",\"price\":99,\"quantity\":1}]}")
  check "7.3 创建商城订单" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"
  ORDER_NO=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('orderNo','') or d.get('id',''))" 2>/dev/null)
  echo "  → orderNo=$ORDER_NO"
fi

R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/orders?userId=$USER_ID&page=1&size=20")
check "7.4 我的订单列表" "0" "$(echo "$R" | json_code)" "total=$(echo "$R" | json_data total)"

# ===== 8. 付费预约对话/站内消息 =====
echo ""
echo "--- 8. 付费预约对话/站内消息 ---"
# 8.1 只有已付费预约可发起对话
if [ -n "$BK_NO" ]; then
  CHAT_MARKER="journey-chat-$(date +%s)-$RANDOM"
  R=$(curl -s --max-time 15 -X POST -H "$AUTH" -H "Content-Type: application/json" \
    "$GATEWAY/api/v1/bookings/$BK_NO/chat/messages" \
    -d "{\"clientMessageId\":\"$CHAT_MARKER\",\"content\":\"你好，我想咨询预约准备事项\"}")
  check "8.1 已付费预约发送文字消息" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"
  R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/bookings/$BK_NO/chat/messages?page=1&size=100")
  check "8.2 对话历史按预约恢复" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"
fi

# 8.3 旧咨询入口固定关闭，不允许绕过付费校验
R=$(curl -s --max-time 10 -X POST -H "$AUTH" -H "Content-Type: application/json" \
  "$GATEWAY/api/v1/messages/send" \
  -d "{\"conversationId\":\"master:1\",\"userId\":\"$USER_ID\",\"content\":\"不应送达\"}")
check "8.3 旧咨询入口拒绝" "40909" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 300)"

# 8.4 未读消息数
R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/messages/unread-count?userId=$USER_ID")
check "8.4 未读消息数" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 200)"

# 8.5 站内消息列表
R=$(curl -s --max-time 10 -H "$AUTH" "$GATEWAY/api/v1/messages/list?userId=$USER_ID&page=1&size=20")
check "8.5 站内消息列表" "0" "$(echo "$R" | json_code)" "total=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('total',0))" 2>/dev/null)"

# 8.6 标记全部已读
R=$(curl -s --max-time 10 -X PUT -H "$AUTH" -H "Content-Type: application/json" \
  "$GATEWAY/api/v1/messages/read-all" \
  -d "{\"userId\":\"$USER_ID\"}")
check "8.6 标记全部已读" "0" "$(echo "$R" | json_code)" "$(echo "$R" | head -c 200)"

# 8.7 公告列表
R=$(curl -s --max-time 10 "$GATEWAY/api/v1/announcements/list?page=1&size=10")
check "8.7 公告列表" "0" "$(echo "$R" | json_code)" ""

# ===== 汇总 =====
echo ""
echo "=========================================="
echo "  C 端用户全流程闭环测试汇总"
echo "=========================================="
TOTAL=$((PASS+FAIL))
echo "  通过: $PASS / $TOTAL"
echo "  失败: $FAIL / $TOTAL"
if [ "$FAIL" -gt 0 ]; then
  echo ""
  echo "--- 失败项 ---"
  for r in "${RESULTS[@]}"; do
    case "$r" in
      FAIL*) echo "  $r" ;;
    esac
  done
fi
echo "=========================================="
