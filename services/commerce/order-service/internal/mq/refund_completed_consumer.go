package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/askxuan/order-service/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 退款完成事件相关常量
const (
	// MessageTypeRefundRequest order→payment 退款请求消息类型（写入 outbox.message_type）
	MessageTypeRefundRequest = "refund.request"

	// QueueOrderRefundCompleted order 消费 payment 退款完成事件的队列名
	// 复用 payment.events exchange（与 payment.notify 同源），独立队列避免与支付通知 handler 耦合
	QueueOrderRefundCompleted = "order.refund.completed"
)

// RefundCompletedEvent 退款完成事件。
// 当前 payment-service 在退款完成时通过 payment.events exchange 发布 PaymentNotify
// （action=refunded），order-service 兼容该格式；同时预留 refundNo 字段，
// 供 payment-service 后续按 outbox 模式发送更结构化的事件。
type RefundCompletedEvent struct {
	PaymentNo string  `json:"paymentNo"`          // 支付单号
	OrderType string  `json:"orderType"`          // 订单类型 shop_order/booking/diy_order
	OrderNo   string  `json:"orderNo"`            // 业务订单号
	Amount    float64 `json:"amount"`             // 退款金额
	Action    string  `json:"action"`             // 动作：refunded / refund.completed
	RefundNo  string  `json:"refundNo,omitempty"` // 退款单号（可选，幂等去重用）
	Time      string  `json:"time"`               // 事件时间
}

// RefundCompletedDeps 退款完成 handler 的依赖。
// 通过显式注入依赖避免 mq ↔ svc 循环依赖。
type RefundCompletedDeps struct {
	FindRefunding    func(context.Context, int64) (*model.ReturnOrder, error)
	Finalize         func(context.Context, *model.ShopOrder, *model.ReturnOrder) error
	ShopOrderModel   model.ShopOrderModel
	ReturnOrderModel model.ReturnOrderModel
	Redis            *redis.Redis
}

// NewRefundCompletedHandler 返回退款完成事件的 handler。
//
// 幂等策略：
//  1. 按业务订单定位 refunding 退货单，避免全局分页遗漏。
//  2. Finalize 使用库存幂等释放与数据库条件更新，事务失败时允许消息重投。
//
// 关联退货单的方式：通过 OrderNo 关联到 shop_order，再通过 shop_order.id
// 关联到 return_order.order_id。一个订单可能有多个退货单，这里只流转最近一笔 refunding 状态的退货单。
func NewRefundCompletedHandler(deps RefundCompletedDeps) func([]byte) error {
	return func(body []byte) error {
		ctx := context.Background()

		var evt RefundCompletedEvent
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 refund.completed 失败，丢弃: %v", err)
			return nil
		}
		logx.Infof("收到退款完成通知: paymentNo=%s orderType=%s orderNo=%s action=%s amount=%.2f",
			evt.PaymentNo, evt.OrderType, evt.OrderNo, evt.Action, evt.Amount)

		// 仅处理商城订单的退款完成事件
		if evt.OrderType != "shop_order" {
			return nil
		}
		if evt.Action != "refunded" && evt.Action != "refund.completed" {
			return nil
		}
		if evt.OrderNo == "" {
			logx.Errorf("退款完成通知缺少 orderNo，丢弃: %+v", evt)
			return nil
		}

		// 通过订单号找到对应的 shop_order，再找关联的 refunding 退货单
		o, err := deps.ShopOrderModel.FindByOrderNo(ctx, evt.OrderNo)
		if err != nil {
			logx.Errorf("退款完成通知：查找订单失败 orderNo=%s: %v", evt.OrderNo, err)
			return err
		}

		var target *model.ReturnOrder
		if deps.FindRefunding != nil {
			target, err = deps.FindRefunding(ctx, o.Id)
			if err != nil {
				return err
			}
		} else {
			// Compatibility for callers without a scoped query: scan every page.
			for page := 1; target == nil; page++ {
				list, total, err := deps.ReturnOrderModel.FindList(ctx, model.ReturnStatusRefunding, page, 50)
				if err != nil {
					return err
				}
				for _, r := range list {
					if r.OrderId == o.Id {
						target = r
						break
					}
				}
				if len(list) < 50 || int64(page*50) >= total {
					break
				}
			}
		}
		if target == nil {
			logx.Infof("退款完成通知：未找到订单 %s 关联的 refunding 退货单，可能已处理", evt.OrderNo)
			return nil
		}

		if deps.Finalize != nil {
			return deps.Finalize(ctx, o, target)
		}
		if _, err := deps.ReturnOrderModel.UpdateStatus(ctx, target.Id, model.ReturnStatusCompleted); err != nil {
			return err
		}
		logx.Infof("退款完成通知：退货单 %s 已流转到 completed", target.ReturnNo)
		return nil
	}
}

// BuildRefundRequestPayload 构造写入 outbox 的 refund.request 消息体。
// 此消息会被 payment-service 消费（或通过 gateway 转发到 payment-service 退款接口）。
//
// 生产调用传入真实 orderNo，由 payment-service 查询关联支付单；
// 不传 orderNo 的旧调用保持兼容。
func BuildRefundRequestPayload(r *model.ReturnOrder, amount float64, orderNo ...string) string {
	evt := map[string]interface{}{
		"returnNo":  r.ReturnNo,
		"returnId":  r.Id,
		"orderId":   r.OrderId,
		"paymentNo": fmt.Sprintf("PAY-%d", r.OrderId),
		"orderType": "shop_order",
		"amount":    amount,
		"reason":    r.Reason,
		"time":      time.Now().Format("2006-01-02 15:04:05"),
	}
	if len(orderNo) > 0 {
		evt["orderNo"] = orderNo[0]
		evt["paymentNo"] = ""
	}
	body, _ := json.Marshal(evt)
	return string(body)
}
