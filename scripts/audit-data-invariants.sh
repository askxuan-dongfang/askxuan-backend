#!/bin/bash

set -euo pipefail

MYSQL_CONTAINER="${MYSQL_CONTAINER:-askxuan-mysql}"
MYSQL_USER="${MYSQL_USER:-root}"
MYSQL_PASSWORD="${MYSQL_PASSWORD:-root123}"
passed=0
failed=0

query_count() {
  docker exec "${MYSQL_CONTAINER}" mysql -N -u"${MYSQL_USER}" -p"${MYSQL_PASSWORD}" \
    -e "$1" 2>/dev/null | tail -n 1
}

check_zero() {
  local name="$1"
  local sql="$2"
  local count
  count="$(query_count "${sql}")"
  if [[ "${count}" == "0" ]]; then
    printf '[PASS] %s\n' "${name}"
    passed=$((passed + 1))
  else
    printf '[FAIL] %s: %s 条异常数据\n' "${name}" "${count}"
    failed=$((failed + 1))
  fi
}

check_zero "预约金额=服务费+功德金" \
  "SELECT COUNT(*) FROM askxuan_booking.booking WHERE ABS(total_fee-service_fee-merit_money)>0.009;"
check_zero "预约金额均非负" \
  "SELECT COUNT(*) FROM askxuan_booking.booking WHERE service_fee<0 OR merit_money<0 OR total_fee<0;"
check_zero "时段库存不越界" \
  "SELECT COUNT(*) FROM askxuan_booking.booking_slot_inventory WHERE capacity<0 OR reserved_count<0 OR reserved_count>capacity;"
check_zero "预约占位均有关联库存" \
  "SELECT COUNT(*) FROM askxuan_booking.booking b LEFT JOIN askxuan_booking.booking_slot_inventory i ON i.temple_code=b.temple_code AND i.service_code=b.service_code AND i.booking_date=b.booking_date AND i.slot_code=b.slot_code WHERE b.slot_reserved=1 AND i.id IS NULL;"
check_zero "库存占位数与预约一致" \
  "SELECT COUNT(*) FROM askxuan_booking.booking_slot_inventory i LEFT JOIN (SELECT temple_code,service_code,booking_date,slot_code,COUNT(*) reserved FROM askxuan_booking.booking WHERE slot_reserved=1 GROUP BY temple_code,service_code,booking_date,slot_code) b ON b.temple_code=i.temple_code AND b.service_code=i.service_code AND b.booking_date=i.booking_date AND b.slot_code=i.slot_code WHERE i.reserved_count<>COALESCE(b.reserved,0);"
check_zero "商城订单明细无孤儿记录" \
  "SELECT COUNT(*) FROM askxuan_order.shop_order_item i LEFT JOIN askxuan_order.shop_order o ON o.id=i.order_id WHERE o.id IS NULL;"
check_zero "商城订单金额等于明细金额" \
  "SELECT COUNT(*) FROM askxuan_order.shop_order o LEFT JOIN (SELECT order_id,SUM(price*quantity) amount,COUNT(*) item_count FROM askxuan_order.shop_order_item GROUP BY order_id) i ON i.order_id=o.id WHERE COALESCE(i.item_count,0)=0 OR ABS(o.total_amount-COALESCE(i.amount,0))>0.009;"
check_zero "DIY订单金额=材料费+加持费" \
  "SELECT COUNT(*) FROM askxuan_diy.diy_order WHERE ABS(total_fee-material_fee-bless_fee)>0.009 OR material_fee<0 OR bless_fee<0 OR total_fee<0;"
check_zero "加持任务均关联DIY订单" \
  "SELECT COUNT(*) FROM askxuan_diy.blessing_task t LEFT JOIN askxuan_diy.diy_order o ON o.order_no=t.diy_order_no WHERE o.id IS NULL;"
check_zero "已完成加持均有凭证" \
  "SELECT COUNT(*) FROM askxuan_diy.blessing_task WHERE status='completed' AND (certificate_urls IS NULL OR certificate_urls='' OR certificate_urls='[]');"
check_zero "成功支付金额匹配业务订单" \
  "SELECT COUNT(*) FROM askxuan_payment.payment p LEFT JOIN askxuan_booking.booking b ON p.order_type='booking' AND b.booking_no=p.order_no LEFT JOIN askxuan_order.shop_order s ON p.order_type='shop_order' AND s.order_no=p.order_no LEFT JOIN askxuan_diy.diy_order d ON p.order_type='diy_order' AND d.order_no=p.order_no WHERE p.status='success' AND ((p.order_type='booking' AND (b.id IS NULL OR ABS(p.amount-b.total_fee)>0.009)) OR (p.order_type='shop_order' AND (s.id IS NULL OR ABS(p.amount-s.pay_amount)>0.009)) OR (p.order_type='diy_order' AND (d.id IS NULL OR ABS(p.amount-d.total_fee)>0.009)) OR p.order_type NOT IN ('booking','shop_order','diy_order'));"
check_zero "商品和材料价格库存均非负" \
  "SELECT (SELECT COUNT(*) FROM askxuan_product.product WHERE price<0 OR market_price<0 OR stock<0)+(SELECT COUNT(*) FROM askxuan_product.product_sku WHERE price<0 OR stock<0)+(SELECT COUNT(*) FROM askxuan_diy.material WHERE unit_price<0 OR stock<0)+(SELECT COUNT(*) FROM askxuan_diy.material_sku WHERE price<0 OR stock<0);"
check_zero "社区评论状态合法" \
  "SELECT COUNT(*) FROM askxuan_community.post_comment WHERE status NOT IN ('pending','approved','rejected');"

printf '数据一致性审计: %d 通过, %d 失败\n' "${passed}" "${failed}"
[[ "${failed}" -eq 0 ]]
