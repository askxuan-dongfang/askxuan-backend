#!/bin/bash
# OpenIM 聊天集成后端验证脚本
GATEWAY="http://127.0.0.1:8080"
OPENIM="http://127.0.0.1:10002"
PASS=0; FAIL=0

check() {
  local name="$1" expected="$2" actual="$3" detail="$4"
  if [ "$actual" = "$expected" ]; then
    PASS=$((PASS+1)); printf "  ✓ %s\n" "$name"
  else
    FAIL=$((FAIL+1)); printf "  ✗ %s [期望:%s 实际:%s] %s\n" "$name" "$expected" "$actual" "$detail"
  fi
}

echo "===== OpenIM 聊天集成后端验证 ====="

# 1. OpenIM 服务可达
echo "--- 1. OpenIM 服务可达性 ---"
R=$(curl -s --max-time 5 -X POST "$OPENIM/auth/get_admin_token" -H "Content-Type: application/json" -H "operationID: test-001" -d '{"secret":"openIM123","userID":"imAdmin"}')
ADMIN_TOKEN=$(echo "$R" | python3 -c "import sys,json;d=json.load(sys.stdin);print(d.get('data',{}).get('token','') or d.get('token',''))" 2>/dev/null)
check "OpenIM admin token 获取" "non-empty" "${ADMIN_TOKEN:+non-empty}" ""

# 2. C 端登录返回 imToken
echo "--- 2. C 端登录 imToken ---"
R=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/login" -H "Content-Type: application/json" -d '{"phone":"13800138000","password":"123456"}')
C_CODE=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',''))" 2>/dev/null)
C_IMTOKEN=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('imToken',''))" 2>/dev/null)
check "C 端登录成功" "0" "$C_CODE" ""
check "C 端 imToken 非空" "non-empty" "${C_IMTOKEN:+non-empty}" ""

# 3. 法师端登录返回 imToken
echo "--- 3. 法师端登录 imToken ---"
R=$(curl -s --max-time 10 -X POST "$GATEWAY/api/v1/auth/admin/login" -H "Content-Type: application/json" -d '{"account":"zhihai","password":"123456"}')
M_CODE=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',''))" 2>/dev/null)
M_IMTOKEN=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('data',{}).get('imToken',''))" 2>/dev/null)
check "法师端登录成功" "0" "$M_CODE" ""
check "法师端 imToken 非空" "non-empty" "${M_IMTOKEN:+non-empty}" ""

# 4. 网关 /api/v1/im 白名单（用 POST 方法，避免 OpenIM 不支持 GET 返回 404 误判）
echo "--- 4. 网关白名单 ---"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 -X POST "$GATEWAY/api/v1/im/user/user_register" -H "Content-Type: application/json" -H "operationID: test" -d '{}')
if [ "$HTTP_CODE" != "401" ]; then
  PASS=$((PASS+1)); printf "  ✓ /api/v1/im 白名单生效（HTTP %s，非 401）\n" "$HTTP_CODE"
else
  FAIL=$((FAIL+1)); printf "  ✗ /api/v1/im 白名单未生效（HTTP %s）\n" "$HTTP_CODE"
fi

# 5. webhook handler 可达
echo "--- 5. webhook handler ---"
R=$(curl -s --max-time 5 -X POST "$GATEWAY/openim/webhook" -H "Content-Type: application/json" -d '{"sendID":"u_test","recvID":"m_test","content":"test","sessionType":1,"contentType":101}')
WH_CODE=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('code',''))" 2>/dev/null)
check "webhook handler 响应" "0" "$WH_CODE" ""

# 6. 消息落库验证
echo "--- 6. 消息落库 ---"
CNT=$(docker exec askxuan-mysql mysql -uroot -proot123 -N -e "SELECT COUNT(*) FROM askxuan_message.message WHERE biz_type='consult' AND biz_id='u_test';" 2>/dev/null)
if [ "$CNT" -ge 1 ] 2>/dev/null; then
  PASS=$((PASS+1)); printf "  ✓ webhook 消息落库（%s 条）\n" "$CNT"
else
  FAIL=$((FAIL+1)); printf "  ✗ webhook 消息落库 [期望>=1 实际:%s]\n" "$CNT"
fi

# 7. OpenIM 用户注册验证（C 端登录时应已注册 u_1）
echo "--- 7. OpenIM 用户注册 ---"
R=$(curl -s --max-time 5 -X POST "$OPENIM/user/get_users_info" -H "Content-Type: application/json" -H "operationID: test-002" -H "token: $ADMIN_TOKEN" -d '{"userIDs":["u_1","m_1"]}')
ERR_CODE=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('errCode',''))" 2>/dev/null)
check "OpenIM 用户 u_1/m_1 已注册" "0" "$ERR_CODE" ""

# 8. 真实 OpenIM 单聊发送后触发 afterSendSingleMsg 回调并落库
echo "--- 8. OpenIM 真实消息闭环 ---"
MARKER="openim_e2e_$(date +%s)"
R=$(curl -s --max-time 10 -X POST "$OPENIM/msg/send_msg" \
  -H "Content-Type: application/json" \
  -H "operationID: test-send-$MARKER" \
  -H "token: $ADMIN_TOKEN" \
  -d "{\"sendID\":\"u_1\",\"recvID\":\"m_1\",\"senderNickname\":\"OpenIM测试用户\",\"senderPlatformID\":1,\"content\":{\"content\":\"$MARKER\"},\"contentType\":101,\"sessionType\":1}")
ERR_CODE=$(echo "$R" | python3 -c "import sys,json;print(json.load(sys.stdin).get('errCode',''))" 2>/dev/null)
check "OpenIM send_msg 成功" "0" "$ERR_CODE" ""

REAL_CNT=0
for _ in $(seq 1 10); do
  REAL_CNT=$(docker exec askxuan-mysql mysql -uroot -proot123 -N -e \
    "SELECT COUNT(*) FROM askxuan_message.message WHERE biz_type='consult' AND biz_id='u_1' AND user_id='m_1' AND content LIKE '%$MARKER%';" 2>/dev/null)
  if [ "$REAL_CNT" -ge 1 ] 2>/dev/null; then
    break
  fi
  sleep 1
done
if [ "$REAL_CNT" -ge 1 ] 2>/dev/null; then
  PASS=$((PASS+1)); printf "  ✓ OpenIM afterSendSingleMsg 已落库\n"
else
  FAIL=$((FAIL+1)); printf "  ✗ OpenIM afterSendSingleMsg 未落库\n"
fi

echo ""
echo "===== 测试汇总: $PASS 通过 / $FAIL 失败 ====="
exit $FAIL
