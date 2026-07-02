package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/askxuan/message-service/internal/config"
	"github.com/askxuan/message-service/internal/handler"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/mq"
	"github.com/askxuan/message-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/message.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	// 启动 RabbitMQ 消费者：监听多个事件交换机，生成站内消息存入 MySQL
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startConsumers(ctx, svcCtx)

	// 优雅退出
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	fmt.Printf("启动 message-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startConsumers 启动所有事件消费者
// 消费 8 个交换机的事件，将每个事件转化为一条站内消息存入 message 表
func startConsumers(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx.Consumer == nil {
		return
	}

	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangeBookingEvents,
			Queue:    "message.booking",
			Handler:  handleBookingNotify(svcCtx),
		},
		{
			Exchange: mq.ExchangeOrderEvents,
			Queue:    "message.order",
			Handler:  handleOrderNotify(svcCtx),
		},
		{
			Exchange: mq.ExchangePaymentEvents,
			Queue:    "message.payment",
			Handler:  handlePaymentNotify(svcCtx),
		},
		{
			Exchange: mq.ExchangeBlessingEvents,
			Queue:    "message.blessing",
			Handler:  handleBlessingComplete(svcCtx),
		},
		{
			Exchange: mq.ExchangeReviewEvents,
			Queue:    "message.review",
			Handler:  handleReviewNotify(svcCtx),
		},
		{
			Exchange: mq.ExchangeAuditEvents,
			Queue:    "message.audit",
			Handler:  handleAuditResult(svcCtx),
		},
		{
			Exchange: mq.ExchangeFinanceEvents,
			Queue:    "message.finance",
			Handler:  handleWithdrawalNotify(svcCtx),
		},
		{
			Exchange: mq.ExchangeLogisticsEvents,
			Queue:    "message.logistics",
			Handler:  handleLogisticsSync(svcCtx),
		},
	}

	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("message-service 已启动 8 个事件消费者")
}

// ===== 事件处理函数（闭包模式，通过 svcCtx 注入 MySQL 持久化）=====

func handleBookingNotify(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		var evt mq.BookingNotify
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 booking.notify 失败，丢弃: %v body=%s", err, string(body))
			return nil
		}
		title, content := buildBookingMessage(evt.Action, evt.BookingId)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			UserId:  evt.UserId,
			Title:   title,
			Content: content,
			BizType: "booking",
			BizId:   evt.BookingId,
		})
		logx.Infof("消费 booking 事件生成站内消息: bookingId=%s action=%s userId=%s", evt.BookingId, evt.Action, evt.UserId)
		return nil
	}
}

func handleOrderNotify(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		var evt mq.OrderNotify
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 order.status 失败，丢弃: %v body=%s", err, string(body))
			return nil
		}
		title, content := buildOrderMessage(evt.Action, evt.OrderId)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			UserId:  evt.UserId,
			Title:   title,
			Content: content,
			BizType: "order",
			BizId:   evt.OrderId,
		})
		logx.Infof("消费 order 事件生成站内消息: orderId=%s action=%s userId=%s", evt.OrderId, evt.Action, evt.UserId)
		return nil
	}
}

func handlePaymentNotify(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		var evt mq.PaymentNotify
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 payment.notify 失败，丢弃: %v body=%s", err, string(body))
			return nil
		}
		title, content := buildPaymentMessage(evt.Action, evt.OrderNo, evt.Amount)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			UserId:  evt.UserId,
			Title:   title,
			Content: content,
			BizType: "payment",
			BizId:   evt.PaymentNo,
		})
		logx.Infof("消费 payment 事件生成站内消息: paymentNo=%s action=%s orderNo=%s userId=%s", evt.PaymentNo, evt.Action, evt.OrderNo, evt.UserId)
		return nil
	}
}

func handleBlessingComplete(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		evt, ok := mq.ParseBlessingComplete(body)
		if !ok {
			return nil // 非 blessing.complete 事件，跳过
		}
		title, content := buildBlessingMessage(evt.Status, evt.TaskNo)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			Title:   title,
			Content: content,
			BizType: "blessing",
			BizId:   evt.TaskNo,
		})
		logx.Infof("消费 blessing.complete 事件生成站内消息: taskNo=%s status=%s", evt.TaskNo, evt.Status)
		return nil
	}
}

func handleReviewNotify(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		var evt mq.ReviewNotify
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 review.notify 失败，丢弃: %v body=%s", err, string(body))
			return nil
		}
		title, content := buildReviewMessage(evt.Action, evt.TargetType, evt.ReviewId)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			UserId:  evt.UserId,
			Title:   title,
			Content: content,
			BizType: "review",
			BizId:   evt.ReviewId,
		})
		logx.Infof("消费 review 事件生成站内消息: reviewId=%s action=%s targetType=%s", evt.ReviewId, evt.Action, evt.TargetType)
		return nil
	}
}

func handleAuditResult(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		var evt mq.AuditResult
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 audit.result 失败，丢弃: %v body=%s", err, string(body))
			return nil
		}
		title, content := buildAuditMessage(evt.Result, evt.BizType, evt.AuditId)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			UserId:  evt.UserId,
			Title:   title,
			Content: content,
			BizType: "audit",
			BizId:   evt.AuditId,
		})
		logx.Infof("消费 audit 事件生成站内消息: auditId=%s result=%s bizType=%s", evt.AuditId, evt.Result, evt.BizType)
		return nil
	}
}

func handleWithdrawalNotify(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		var evt mq.WithdrawalNotify
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 withdrawal.notify 失败，丢弃: %v body=%s", err, string(body))
			return nil
		}
		title, content := buildWithdrawalMessage(evt.Status, evt.WithdrawalId, evt.Amount)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			UserId:  evt.UserId,
			Title:   title,
			Content: content,
			BizType: "withdrawal",
			BizId:   evt.WithdrawalId,
		})
		logx.Infof("消费 withdrawal 事件生成站内消息: withdrawalId=%s status=%s userId=%s", evt.WithdrawalId, evt.Status, evt.UserId)
		return nil
	}
}

func handleLogisticsSync(svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		var evt mq.LogisticsSync
		if err := json.Unmarshal(body, &evt); err != nil {
			logx.Errorf("解析 logistics.sync 失败，丢弃: %v body=%s", err, string(body))
			return nil
		}
		title, content := buildLogisticsMessage(evt.Status, evt.OrderId, evt.ExpressNo)
		_, _ = svcCtx.MessageModel.Insert(context.Background(), &model.Message{
			Title:   title,
			Content: content,
			BizType: "logistics",
			BizId:   evt.OrderId,
		})
		logx.Infof("消费 logistics 事件生成站内消息: orderId=%s status=%s expressNo=%s", evt.OrderId, evt.Status, evt.ExpressNo)
		return nil
	}
}

// ===== 消息内容构建函数 =====

func buildBookingMessage(action, bookingId string) (string, string) {
	switch action {
	case "created":
		return "预约已创建", fmt.Sprintf("您的预约（单号 %s）已提交，请等待寺院确认。", bookingId)
	case "confirmed":
		return "预约已确认", fmt.Sprintf("您的预约（单号 %s）已被寺院确认，请按时到达。", bookingId)
	case "inProgress":
		return "预约进行中", fmt.Sprintf("您的预约（单号 %s）已开始进行。", bookingId)
	case "completed":
		return "预约已完成", fmt.Sprintf("您的预约（单号 %s）已完成，感谢您的功德。", bookingId)
	case "cancelled":
		return "预约已取消", fmt.Sprintf("您的预约（单号 %s）已取消。", bookingId)
	default:
		return "预约状态更新", fmt.Sprintf("您的预约（单号 %s）状态已更新为 %s。", bookingId, action)
	}
}

func buildOrderMessage(action, orderId string) (string, string) {
	switch action {
	case "created":
		return "订单已创建", fmt.Sprintf("您的订单（%s）已创建，请尽快完成支付。", orderId)
	case "paid":
		return "订单已支付", fmt.Sprintf("您的订单（%s）已支付成功，等待商家发货。", orderId)
	case "shipped":
		return "订单已发货", fmt.Sprintf("您的订单（%s）已发货，请注意查收。", orderId)
	case "completed":
		return "订单已完成", fmt.Sprintf("您的订单（%s）已完成，感谢您的惠顾。", orderId)
	case "cancelled":
		return "订单已取消", fmt.Sprintf("您的订单（%s）已取消。", orderId)
	case "return":
		return "退货申请", fmt.Sprintf("您的订单（%s）退货申请已受理。", orderId)
	default:
		return "订单状态更新", fmt.Sprintf("您的订单（%s）状态已更新为 %s。", orderId, action)
	}
}

func buildPaymentMessage(action, orderNo string, amount float64) (string, string) {
	switch action {
	case "success":
		return "支付成功", fmt.Sprintf("订单 %s 支付成功，金额 ¥%.2f。", orderNo, amount)
	case "failed":
		return "支付失败", fmt.Sprintf("订单 %s 支付失败，请重试。", orderNo)
	case "refunded":
		return "已退款", fmt.Sprintf("订单 %s 已退款，金额 ¥%.2f。", orderNo, amount)
	default:
		return "支付状态更新", fmt.Sprintf("订单 %s 支付状态: %s。", orderNo, action)
	}
}

func buildBlessingMessage(status, taskNo string) (string, string) {
	switch status {
	case "completed":
		return "加持已完成", fmt.Sprintf("您的加持任务（%s）已完成。", taskNo)
	case "failed":
		return "加持失败", fmt.Sprintf("您的加持任务（%s）执行失败，请联系客服。", taskNo)
	default:
		return "加持状态更新", fmt.Sprintf("您的加持任务（%s）状态: %s。", taskNo, status)
	}
}

func buildReviewMessage(action, targetType, reviewId string) (string, string) {
	switch action {
	case "created":
		return "新评价通知", fmt.Sprintf("您有一条新的评价（%s）待查看。", reviewId)
	case "replied":
		return "评价已回复", fmt.Sprintf("您的评价（%s）已收到回复。", reviewId)
	case "reported":
		return "评价被举报", fmt.Sprintf("评价（%s）已被举报，请及时处理。", reviewId)
	default:
		return "评价通知", fmt.Sprintf("评价（%s）状态更新: %s。", reviewId, action)
	}
}

func buildAuditMessage(result, bizType, auditId string) (string, string) {
	switch result {
	case "approved":
		return "审核通过", fmt.Sprintf("您的%s审核（%s）已通过。", bizType, auditId)
	case "rejected":
		return "审核未通过", fmt.Sprintf("您的%s审核（%s）未通过，请修改后重新提交。", bizType, auditId)
	default:
		return "审核结果通知", fmt.Sprintf("您的%s审核（%s）结果: %s。", bizType, auditId, result)
	}
}

func buildWithdrawalMessage(status, withdrawalId string, amount float64) (string, string) {
	switch status {
	case "approved":
		return "提现审核通过", fmt.Sprintf("您的提现申请（%s）审核已通过，金额 ¥%.2f。", withdrawalId, amount)
	case "rejected":
		return "提现审核未通过", fmt.Sprintf("您的提现申请（%s）审核未通过，请联系客服。", withdrawalId)
	case "paid":
		return "提现已到账", fmt.Sprintf("您的提现（%s）已到账，金额 ¥%.2f。", withdrawalId, amount)
	default:
		return "提现状态更新", fmt.Sprintf("您的提现申请（%s）状态: %s。", withdrawalId, status)
	}
}

func buildLogisticsMessage(status, orderId, expressNo string) (string, string) {
	switch status {
	case "shipped":
		return "订单已发货", fmt.Sprintf("您的订单（%s）已发货，运单号 %s，请注意查收。", orderId, expressNo)
	case "in_transit":
		return "物流运输中", fmt.Sprintf("您的订单（%s）运单 %s 运输中，请耐心等待。", orderId, expressNo)
	case "delivered":
		return "已派送", fmt.Sprintf("您的订单（%s）运单 %s 已派送，请准备签收。", orderId, expressNo)
	case "signed":
		return "已签收", fmt.Sprintf("您的订单（%s）运单 %s 已签收，感谢您的惠顾。", orderId, expressNo)
	default:
		return "物流状态更新", fmt.Sprintf("您的订单（%s）物流状态: %s。", orderId, status)
	}
}
