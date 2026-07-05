package mq

import (
	"context"
	"encoding/json"
	"time"

	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 退款请求相关常量
const (
	// MessageTypeRefundRequest order→payment 退款请求消息类型（写入 outbox.message_type，
	// 对应 amqp.Publishing.Type 字段）
	MessageTypeRefundRequest = "refund.request"

	// QueuePaymentRefundRequest payment 服务消费退款请求的队列名，
	// 绑定到 order.events exchange（fanout），按 MessageTypeRefundRequest 过滤。
	QueuePaymentRefundRequest = "payment.refund.request"
)

// 退款幂等 key 的 TTL
const (
	refundProcessingTTLSec = 3600      // processing 标记 1 小时，防止并发处理与崩溃后死锁
	refundDoneTTLSec       = 7 * 86400 // done 标记 7 天，覆盖业务对账周期
	refundRetryTTLSec      = 3600      // 重试计数 1 小时窗口，超时后清零
	refundMaxRetry         = 3         // 超过 3 次重试后转入死信队列
)

// RefundRequestPayload refund.request 消息体。
// 字段与 order-service 的 BuildRefundRequestPayload 保持一致。
type RefundRequestPayload struct {
	ReturnNo  string  `json:"returnNo"`  // 退货单号
	ReturnId  int64   `json:"returnId"`  // 退货单 ID
	OrderId   int64   `json:"orderId"`   // 业务订单 ID
	PaymentNo string  `json:"paymentNo"` // 支付单号
	OrderType string  `json:"orderType"` // 订单类型 shop_order/booking/diy_order
	Amount    float64 `json:"amount"`    // 退款金额
	Reason    string  `json:"reason"`    // 退款原因
	Time      string  `json:"time"`      // 发起时间
}

// RefundRequestDeps refund.request handler 的依赖。
// 通过显式注入 RefundFunc 避免 mq ↔ logic ↔ svc 循环依赖。
type RefundRequestDeps struct {
	// RefundFunc 执行退款逻辑，返回退款单号。
	// 通常由 main 包装 logic.NewRefundLogic(ctx, svcCtx).Refund(...) 实现。
	RefundFunc func(ctx context.Context, paymentNo string, amount float64, reason string) (refundNo string, err error)

	// Redis 幂等去重（processing / done / retry 计数）
	Redis *redis.Redis

	// MqProducer 发布退款完成事件（可为 nil，nil 时跳过发送）
	MqProducer *Producer
}

// NewRefundRequestHandler 返回 refund.request 消息的 handler。
//
// 幂等策略：
//  1. 收到消息先检查 payment:refund:done:{paymentNo}（TTL 7d），已存在则直接 ACK 不重复处理
//  2. SETNX payment:refund:processing:{paymentNo}（TTL 1h）防止并发处理同一支付单
//  3. 处理完成后写 done 标记，删除 processing 标记
//
// 重试策略：
//  1. 退款失败时 NACK requeue=true 重投
//  2. 用 Redis INCR 记录重试次数，超过 3 次返回 ErrDeadLetter 转入死信队列
//  3. payment 已是 refunded 状态（ErrStatusInvalid）视为幂等成功，不重试
func NewRefundRequestHandler(deps RefundRequestDeps) func([]byte) error {
	return func(body []byte) error {
		ctx := context.Background()

		var payload RefundRequestPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			logx.Errorf("解析 refund.request 失败，丢弃: %v", err)
			return nil // 解析失败不可恢复，ACK 丢弃
		}
		if payload.PaymentNo == "" || payload.Amount <= 0 {
			logx.Errorf("refund.request 缺少必要字段，丢弃: %+v", payload)
			return nil
		}

		logx.Infof("收到退款请求: paymentNo=%s returnNo=%s amount=%.2f reason=%s",
			payload.PaymentNo, payload.ReturnNo, payload.Amount, payload.Reason)

		// 1. 幂等：检查 done 标记，已完成则直接 ACK
		doneKey := "payment:refund:done:" + payload.PaymentNo
		if deps.Redis != nil {
			if val, _ := deps.Redis.Get(doneKey); val != "" {
				logx.Infof("退款已完成，跳过重复消息: paymentNo=%s refundNo=%s", payload.PaymentNo, val)
				return nil
			}
		}

		// 2. 幂等：SETNX processing 标记（TTL 1h），防止并发处理
		processingKey := "payment:refund:processing:" + payload.PaymentNo
		if deps.Redis != nil {
			ok, _ := deps.Redis.SetnxEx(processingKey, "1", refundProcessingTTLSec)
			if !ok {
				// 另一个消费者正在处理同一支付单，ACK 避免重投风暴
				// 若处理方崩溃，processing 标记会在 1h 后过期，届时可重新处理
				logx.Infof("退款正在处理中，跳过: paymentNo=%s", payload.PaymentNo)
				return nil
			}
		}

		// 3. 调用退款逻辑
		refundNo, err := deps.RefundFunc(ctx, payload.PaymentNo, payload.Amount, payload.Reason)

		// 4. 退款失败处理
		if err != nil {
			// 清理 processing 标记，允许重试
			if deps.Redis != nil {
				_, _ = deps.Redis.Del(processingKey)
			}

			// 支付单已是 refunded 状态 → 视为幂等成功（done 标记过期后的重复消息）
			if isAlreadyRefundedErr(err) {
				logx.Infof("退款状态非法（可能已完成），视为幂等成功: paymentNo=%s err=%v",
					payload.PaymentNo, err)
				markRefundDone(deps, doneKey, refundNo)
				publishRefundCompletedEvent(deps, payload, refundNo, "success")
				return nil
			}

			// 其它失败：累加重试计数，超过上限转入死信
			logx.Errorf("退款失败: paymentNo=%s err=%v", payload.PaymentNo, err)
			return handleRefundRetry(deps.Redis, payload.PaymentNo, err)
		}

		// 5. 退款成功：写 done 标记，清理 processing 与重试计数，发送完成事件
		markRefundDone(deps, doneKey, refundNo)
		if deps.Redis != nil {
			_, _ = deps.Redis.Del(processingKey)
			_, _ = deps.Redis.Del("payment:refund:retry:" + payload.PaymentNo)
		}
		publishRefundCompletedEvent(deps, payload, refundNo, "success")
		logx.Infof("退款处理完成: paymentNo=%s refundNo=%s returnNo=%s",
			payload.PaymentNo, refundNo, payload.ReturnNo)
		return nil
	}
}

// isAlreadyRefundedErr 判断错误是否表示支付单已退款（状态流转非法）。
// RefundLogic 在 CanPaymentTransit(refunded→refunding) 失败时返回 common.ErrStatusInvalid。
func isAlreadyRefundedErr(err error) bool {
	if bizErr, ok := err.(*common.BizError); ok {
		return bizErr.Code == common.ErrStatusInvalid.Code
	}
	return false
}

// markRefundDone 标记退款完成（TTL 7d），value 存退款单号便于排查
func markRefundDone(deps RefundRequestDeps, doneKey, refundNo string) {
	if deps.Redis != nil {
		_, _ = deps.Redis.SetnxEx(doneKey, refundNo, refundDoneTTLSec)
	}
}

// handleRefundRetry 累加重试次数，超过上限返回 ErrDeadLetter，否则返回原错误（NACK requeue）
func handleRefundRetry(rds *redis.Redis, paymentNo string, err error) error {
	if rds == nil {
		return err // 无 Redis 时直接 NACK requeue
	}
	retryKey := "payment:refund:retry:" + paymentNo
	count, _ := rds.Incr(retryKey)
	if count == 1 {
		_ = rds.Expire(retryKey, refundRetryTTLSec)
	}
	if count > refundMaxRetry {
		logx.Errorf("退款重试超过 %d 次，转入死信队列: paymentNo=%s", refundMaxRetry, paymentNo)
		_, _ = rds.Del(retryKey)
		return ErrDeadLetter
	}
	return err
}

// publishRefundCompletedEvent 发送退款完成事件到 payment.events exchange。
// RefundLogic 内部已通过 publishPaymentNotify 发送 action=refunded 的 PaymentNotify，
// 这里额外发送结构化的 refund.completed 事件（含 returnNo），供 order-service 后续精确关联退货单。
func publishRefundCompletedEvent(deps RefundRequestDeps, payload RefundRequestPayload, refundNo, status string) {
	if deps.MqProducer == nil {
		return
	}
	evt := RefundCompletedEvent{
		ReturnNo:  payload.ReturnNo,
		PaymentNo: payload.PaymentNo,
		RefundNo:  refundNo,
		Status:    status,
		Amount:    payload.Amount,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := deps.MqProducer.PublishRefundCompleted(context.Background(), evt); err != nil {
		logx.Errorf("发送 refund.completed 事件失败，不影响主流程: %v", err)
	}
}
