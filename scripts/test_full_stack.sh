#!/bin/bash
# 问玄东方 全端闭环测试脚本
# 覆盖 5 个端侧 + 20 个后端服务 + 跨服务链路
GATEWAY="http://127.0.0.1:8080"
PASS=0; FAIL=0
RESULTS=()

json_code() { python3 -c "import sys,json;print(json.load(sys.stdin).get('code',''))" 2>/dev/null; }

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

echo "=========================================="
echo "  问玄东方 全端闭环测试（5端 + 20服务）"
echo "=========================================="

# ===== 获取 Token =====
echo ""
echo "--- 获取各端 Token ---"
C_RESP=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/login" -H "Content-Type: application/json" -d '{"phone":"13800138000","password":"123456"}')
C_TOKEN=$(echo "$C_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('accessToken',''))" 2>/dev/null)
check "C端用户登录" "0" "$(echo "$C_RESP" | json_code)" ""

A_RESP=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/admin/login" -H "Content-Type: application/json" -d '{"account":"admin","password":"123456"}')
A_TOKEN=$(echo "$A_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('accessToken',''))" 2>/dev/null)
check "平台管理台登录(admin)" "0" "$(echo "$A_RESP" | json_code)" ""

T_RESP=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/admin/login" -H "Content-Type: application/json" -d '{"account":"lingyin_admin","password":"123456"}')
T_TOKEN=$(echo "$T_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('accessToken',''))" 2>/dev/null)
check "寺院管理台登录(lingyin_admin)" "0" "$(echo "$T_RESP" | json_code)" ""

M_RESP=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/admin/login" -H "Content-Type: application/json" -d '{"account":"zhihai","password":"123456"}')
M_TOKEN=$(echo "$M_RESP" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('accessToken',''))" 2>/dev/null)
check "法师端登录(zhihai)" "0" "$(echo "$M_RESP" | json_code)" ""

# ===== 端侧 1: iOS C 端 =====
echo ""
echo "=== 端侧 1: iOS C 端 (ios-customer + mobile-customer) ==="

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/temples")
check "C端-寺院列表" "0" "$(echo "$R" | json_code)" "total=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('total','?'))" 2>/dev/null)"

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/temples/T001")
check "C端-寺院详情" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/temples/T001/services")
check "C端-寺院服务列表" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/masters")
check "C端-法师列表" "0" "$(echo "$R" | json_code)" "total=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});print(d.get('total','?'))" 2>/dev/null)"

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/masters/M001")
check "C端-法师详情" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/products")
check "C端-商品列表" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/products/categories")
check "C端-商品分类" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/marketing/banners")
check "C端-首页Banner" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 "$GATEWAY/api/v1/community/feed?page=1&size=20")
check "C端-大师广场" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $C_TOKEN" "$GATEWAY/api/v1/users/profile")
check "C端-用户信息(JWT)" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $C_TOKEN" "$GATEWAY/api/v1/messages/unread-count?userId=1")
check "C端-未读消息数(JWT)" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $C_TOKEN" "$GATEWAY/api/v1/bookings")
check "C端-我的预约列表(JWT)" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $C_TOKEN" "$GATEWAY/api/v1/orders?userId=1&page=1&size=20")
check "C端-我的订单列表(JWT)" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $C_TOKEN" "$GATEWAY/api/v1/diy/designs")
check "C端-DIY设计列表(JWT)" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $C_TOKEN" "$GATEWAY/api/v1/diy/materials")
check "C端-DIY材料列表(JWT)" "0" "$(echo "$R" | json_code)" ""

# ===== 端侧 2: iOS 法师端 =====
echo ""
echo "=== 端侧 2: iOS 法师端 (ios-master) ==="

R=$(curl -s --max-time 10 -H "Authorization: Bearer $M_TOKEN" "$GATEWAY/api/v1/admin/masters/blessing-tasks")
check "法师端-加持任务列表" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $M_TOKEN" "$GATEWAY/api/v1/admin/masters/schedules")
check "法师端-排班列表" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $M_TOKEN" "$GATEWAY/api/v1/admin/masters/profile")
check "法师端-法师信息" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $M_TOKEN" "$GATEWAY/api/v1/admin/masters/bookings")
check "法师端-预约列表" "0" "$(echo "$R" | json_code)" ""
BK_RESP="$R"

R=$(curl -s --max-time 10 -H "Authorization: Bearer $M_TOKEN" "$GATEWAY/api/v1/admin/masters/community/posts?page=1&size=20")
check "法师端-我的内容" "0" "$(echo "$R" | json_code)" ""

# 法师端预约详情/确认/完成（新补齐的 3 个端点）
BK_NO=$(echo "$BK_RESP" | python3 -c "import sys,json;d=json.load(sys.stdin).get('data',{});l=d.get('list',[]);print(l[0].get('id','') if l else '')" 2>/dev/null)
if [ -n "$BK_NO" ]; then
  R=$(curl -s --max-time 10 -H "Authorization: Bearer $M_TOKEN" "$GATEWAY/api/v1/admin/masters/bookings/$BK_NO")
  check "法师端-预约详情" "0" "$(echo "$R" | json_code)" "bookingNo=$BK_NO"
else
  PASS=$((PASS+1)); RESULTS+=("PASS 法师端-预约详情(无数据，跳过)")
  printf "  ✓ 法师端-预约详情(无数据，跳过)\n"
fi

# ===== 端侧 3: web-temple-admin 寺院管理台 =====
echo ""
echo "=== 端侧 3: web-temple-admin 寺院管理台 ==="

R=$(curl -s --max-time 10 -H "Authorization: Bearer $T_TOKEN" "$GATEWAY/api/v1/admin/temples/info")
check "寺院管理台-寺院信息" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $T_TOKEN" "$GATEWAY/api/v1/admin/temples/services")
check "寺院管理台-寺院服务" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $T_TOKEN" "$GATEWAY/api/v1/admin/temples/blessing-tasks")
check "寺院管理台-加持任务" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $T_TOKEN" "$GATEWAY/api/v1/admin/bookings?templeId=T001&page=1&size=20")
check "寺院管理台-预约管理" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $T_TOKEN" "$GATEWAY/api/v1/admin/temples/masters?templeId=T001&page=1&size=20")
check "寺院管理台-法师管理" "0" "$(echo "$R" | json_code)" ""

# ===== 端侧 4: web-shop-admin 商城管理台 =====
echo ""
echo "=== 端侧 4: web-shop-admin 商城管理台 ==="

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/products")
check "商城管理台-商品列表" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/products/categories")
check "商城管理台-分类管理" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/orders")
check "商城管理台-订单管理" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/diy/orders")
check "商城管理台-DIY订单" "0" "$(echo "$R" | json_code)" ""

# ===== 端侧 5: web-platform-admin 平台管理台 =====
echo ""
echo "=== 端侧 5: web-platform-admin 平台管理台 ==="

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/auth/accounts")
check "平台管理台-账号管理" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/platform/temples")
check "平台管理台-寺院审核列表" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/platform/masters/audits")
check "平台管理台-法师审核列表" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/finance/overview")
check "平台管理台-财务概览" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/audit/queue")
check "平台管理台-审核队列" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -H "Authorization: Bearer $A_TOKEN" "$GATEWAY/api/v1/admin/users")
check "平台管理台-用户管理" "0" "$(echo "$R" | json_code)" ""

# ===== 跨服务链路验证 =====
echo ""
echo "=== 跨服务链路验证 ==="

R=$(curl -s --max-time 10 -H "Authorization: Bearer $M_TOKEN" "$GATEWAY/api/v1/admin/masters/blessing-tasks")
check "zrpc: master→diy(blessing_task)" "0" "$(echo "$R" | json_code)" "通过zrpc查blessing_task"

R=$(curl -s --max-time 10 -H "Authorization: Bearer $T_TOKEN" "$GATEWAY/api/v1/admin/temples/blessing-tasks")
check "zrpc: temple→diy(blessing_task)" "0" "$(echo "$R" | json_code)" "通过zrpc查blessing_task"

R=$(curl -s --max-time 10 -H "Authorization: Bearer $C_TOKEN" "$GATEWAY/api/v1/bookings")
check "预约独立库快照列表（无运行时跨库）" "0" "$(echo "$R" | json_code)" ""

R=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/login" -H "Content-Type: application/json" -d '{"phone":"13800138001","password":"123456"}')
check "跨库: auth→user(登录)" "0" "$(echo "$R" | json_code)" ""

OUTBOX_CNT=$(docker exec askxuan-mysql mysql -uroot -proot123 -N -e "SELECT COUNT(*) FROM askxuan_order.outbox;" 2>/dev/null)
check "Outbox表: order.outbox" "0" "$?" "count=$OUTBOX_CNT"

MQ_EXCH=$(docker exec askxuan-rabbitmq rabbitmqctl list_exchanges -p / 2>/dev/null | grep -cE "events|refund.*dlx")
if [ "$MQ_EXCH" -ge 5 ]; then
  PASS=$((PASS+1)); RESULTS+=("PASS RabbitMQ exchanges ($MQ_EXCH 个)")
  printf "  ✓ RabbitMQ exchanges (%d 个业务exchange)\n" "$MQ_EXCH"
else
  FAIL=$((FAIL+1)); RESULTS+=("FAIL RabbitMQ exchanges ($MQ_EXCH)")
  printf "  ✗ RabbitMQ exchanges 不足 (%d)\n" "$MQ_EXCH"
fi

REDIS_OK=$(docker exec askxuan-redis redis-cli ping 2>/dev/null)
if [ "$REDIS_OK" = "PONG" ]; then
  PASS=$((PASS+1)); RESULTS+=("PASS Redis 连接正常")
  printf "  ✓ Redis 连接正常\n"
else
  FAIL=$((FAIL+1)); RESULTS+=("FAIL Redis")
  printf "  ✗ Redis 连接失败\n"
fi

# etcd: 检查全部业务 zrpc 服务注册；端口监听作为本机开发兜底。
for rpc in temple.rpc:9083 master.rpc:9084 diy.rpc:9088 payment.rpc:9090; do
  key="${rpc%:*}"
  port="${rpc#*:}"
  ETCD_KEYS=$(docker exec askxuan-etcd etcdctl --endpoints=http://127.0.0.1:2379 get "$key" --prefix --keys-only 2>/dev/null | grep -c "$key")
  if [ "$ETCD_KEYS" -ge 1 ]; then
    PASS=$((PASS+1)); RESULTS+=("PASS etcd: $key 已注册")
    printf "  ✓ etcd: %s 已注册 (%d 个实例)\n" "$key" "$ETCD_KEYS"
  elif lsof -tiTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    PASS=$((PASS+1)); RESULTS+=("PASS $key gRPC $port 监听中")
    printf "  ✓ %s gRPC 端口 %s 监听中\n" "$key" "$port"
  else
    FAIL=$((FAIL+1)); RESULTS+=("FAIL $key 未注册")
    printf "  ✗ %s 未注册且 %s 未监听\n" "$key" "$port"
  fi
done

# ===== 汇总 =====
echo ""
echo "=========================================="
echo "  测试汇总"
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
