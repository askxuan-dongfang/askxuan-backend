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
	ShopOrderModel   model.ShopOrderModel
	ReturnOrderModel model.ReturnOrderModel
	Redis            *redis.Redis
}

// NewRefundCompletedHandler 返回退款完成事件的 handler。
//
// 幂等策略：
//  1. 用 Redis SETNX 标记 returnNo 已处理（24h TTL），防止同一退款事件重复流转
//  2. 状态机校验：仅 refunding → completed 合法，已完成的状态变更会被 CanReturnTransit 拒绝
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
			// 不重投，避免消息堆积；后续可由对账任务补偿
			return nil
		}

		// 查询该订单下 refunding 状态的退货单
		list, _, err := deps.ReturnOrderModel.FindList(ctx, model.ReturnStatusRefunding, 1, 50)
		if err != nil {
			logx.Errorf("退款完成通知：查询退款中退货单失败: %v", err)
			return nil
		}
		var target *model.ReturnOrder
		for _, r := range list {
			if r.OrderId == o.Id {
				target = r
				break
			}
		}
		if target == nil {
			logx.Infof("退款完成通知：未找到订单 %s 关联的 refunding 退货单，可能已处理", evt.OrderNo)
			return nil
		}

		// 幂等去重：用 return_no 在 Redis 标记，TTL 24h
		idemKey := "order:refund:completed:" + target.ReturnNo
		if deps.Redis != nil {
			ok, _ := deps.Redis.SetnxEx(idemKey, "1", 86400)
			if !ok {
				logx.Infof("退款完成通知：退货单 %s 已处理过，跳过", target.ReturnNo)
				return nil
			}
		}

		// 状态机校验
		if !model.CanReturnTransit(target.Status, model.ReturnStatusCompleted) {
			logx.Infof("退款完成通知：退货单 %s 状态 %s 无法流转到 completed，跳过",
				target.ReturnNo, target.Status)
			return nil
		}

		if _, err := deps.ReturnOrderModel.UpdateStatus(ctx, target.Id, model.ReturnStatusCompleted); err != nil {
			logx.Errorf("退款完成通知：更新退货单状态失败 returnNo=%s: %v", target.ReturnNo, err)
			// 状态更新失败则回滚幂等标记，允许下次重试
			if deps.Redis != nil {
				_, _ = deps.Redis.Del(idemKey)
			}
			return err
		}
		logx.Infof("退款完成通知：退货单 %s 已流转到 completed", target.ReturnNo)
		return nil
	}
}

// BuildRefundRequestPayload 构造写入 outbox 的 refund.request 消息体。
// 此消息会被 payment-service 消费（或通过 gateway 转发到 payment-service 退款接口）。
//
// 注意：paymentNo 当前仍按 callPaymentRefund 的临时构造方式（PAY-{orderId}），
// 待 payment-service 提供按 orderNo 查询支付单的能力后可移除该字段。
func BuildRefundRequestPayload(r *model.ReturnOrder, amount float64) string {
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
	body, _ := json.Marshal(evt)
	return string(body)
}
