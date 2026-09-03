package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/askxuan/finance-service/internal/config"
	"github.com/askxuan/finance-service/internal/handler"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/mq"
	"github.com/askxuan/finance-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/finance.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	// 启动 RabbitMQ 消费者：监听 booking.status / order.status / payment.notify
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startConsumer(ctx, svcCtx)

	// 优雅退出
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	fmt.Printf("启动 finance-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startConsumer 启动财务相关事件消费者
func startConsumer(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx == nil || svcCtx.Consumer == nil {
		return
	}
	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangeConsultationEvents,
			Queue:    mq.QueueFinanceConsultationStatus,
			Handler: func(body []byte) error {
				var evt mq.ConsultationNotify
				if err := json.Unmarshal(body, &evt); err != nil {
					return nil
				}
				if evt.Action != "paid" {
					return nil
				}
				earningDate := evt.Time
				if len(earningDate) >= 10 {
					earningDate = earningDate[:10]
				}
				split, err := model.AccrueConsultationSettlement(ctx, model.ConsultationSettlement{
					ConsultationID: evt.ConsultationId, UserID: evt.UserId, MasterID: evt.MasterId,
					MasterName: evt.MasterName, PaidAt: evt.Time, ConsultFee: evt.ConsultFee,
				})
				if err != nil {
					return fmt.Errorf("即时咨询平台分账失败: %w", err)
				}
				if split.MasterNet > 0 && svcCtx.MqProducer != nil {
					if err := svcCtx.MqProducer.PublishSettlementAccrued(ctx, mq.SettlementAccrued{
						SourceType: model.BizTypeConsultation, SourceNo: evt.ConsultationId,
						TargetType: model.SettleTypeMaster, TargetId: evt.MasterId, UserId: evt.UserId,
						ServiceName: "即时文字咨询", EarningDate: earningDate, Amount: split.MasterNet,
					}); err != nil {
						return fmt.Errorf("发布咨询分成事件失败: %w", err)
					}
				}
				logx.Infof("即时咨询平台分账完成: consultation=%s gross=%.2f commission=%.2f master=%.2f", evt.ConsultationId, split.Total, split.Commission, split.MasterNet)
				return nil
			},
		},
		{
			Exchange: mq.ExchangeBookingEvents,
			Queue:    mq.QueueFinanceBookingStatus,
			Handler: func(body []byte) error {
				var evt mq.BookingNotify
				if err := json.Unmarshal(body, &evt); err != nil {
					logx.Errorf("解析 booking.status 失败，丢弃: %v", err)
					return nil
				}
				logx.Infof("收到预约状态变更: bookingId=%s action=%s userId=%s",
					evt.BookingId, evt.Action, evt.UserId)
				// 预约已评价结案后，平台总账才从托管资金中确认抽成和两方应付。
				if evt.Action == "reviewed" {
					split, err := model.AccrueBookingSettlement(ctx, model.BookingSettlement{
						BookingID: evt.BookingId, UserID: evt.UserId,
						TempleID: evt.TempleId, TempleName: evt.TempleName,
						MasterID: evt.MasterId, MasterName: evt.MasterName,
						ServiceName: evt.ServiceName, BookingDate: evt.BookingDate,
						ServiceFee: evt.ServiceFee, MeritMoney: evt.MeritMoney, TotalFee: evt.TotalFee,
					})
					if err != nil {
						return fmt.Errorf("预约平台分账失败: %w", err)
					}
					if split.MasterNet > 0 && svcCtx.MqProducer != nil {
						if err := svcCtx.MqProducer.PublishSettlementAccrued(ctx, mq.SettlementAccrued{
							SourceType: model.BizTypeBooking, SourceNo: evt.BookingId,
							TargetType: model.SettleTypeMaster, TargetId: evt.MasterId,
							UserId: evt.UserId, ServiceName: evt.ServiceName,
							EarningDate: evt.BookingDate, Amount: split.MasterNet,
						}); err != nil {
							return fmt.Errorf("发布大师分成事件失败: %w", err)
						}
					}
					logx.Infof("预约平台分账完成: bookingId=%s gross=%.2f commission=%.2f master=%.2f temple=%.2f created=%t",
						evt.BookingId, split.Total, split.Commission, split.MasterNet, split.TempleNet, split.Created)
				}
				return nil
			},
		},
		{
			Exchange: mq.ExchangeOrderEvents,
			Queue:    mq.QueueFinanceOrderStatus,
			Handler: func(body []byte) error {
				var evt mq.OrderNotify
				if err := json.Unmarshal(body, &evt); err != nil {
					logx.Errorf("解析 order.status 失败，丢弃: %v", err)
					return nil
				}
				logx.Infof("收到订单状态变更: orderId=%s action=%s userId=%s",
					evt.OrderId, evt.Action, evt.UserId)
				if evt.Action == "completed" {
					model.InsertFinanceLog(model.FinanceLog{
						Type:        "income",
						Amount:      0,
						Description: fmt.Sprintf("商城订单收入:%s", evt.OrderId),
					})
				}
				return nil
			},
		},
		{
			Exchange: mq.ExchangePaymentEvents,
			Queue:    mq.QueueFinancePaymentNotify,
			Handler: func(body []byte) error {
				var evt mq.PaymentNotify
				if err := json.Unmarshal(body, &evt); err != nil {
					logx.Errorf("解析 payment.notify 失败，丢弃: %v", err)
					return nil
				}
				logx.Infof("收到支付通知: paymentNo=%s orderType=%s orderNo=%s action=%s amount=%.2f",
					evt.PaymentNo, evt.OrderType, evt.OrderNo, evt.Action, evt.Amount)
				if evt.Action == "success" {
					if err := model.RecordPlatformReceipt(ctx, model.PaymentReceipt{
						PaymentNo: evt.PaymentNo, SourceType: evt.OrderType,
						SourceNo: evt.OrderNo, Amount: evt.Amount,
					}); err != nil {
						return fmt.Errorf("支付进入平台总账失败: %w", err)
					}
				} else if evt.Action == "refunded" {
					if err := model.RecordPlatformRefund(ctx, model.PaymentReceipt{
						PaymentNo: evt.PaymentNo, SourceType: evt.OrderType,
						SourceNo: evt.OrderNo, Amount: evt.Amount,
					}); err != nil {
						return fmt.Errorf("退款冲销平台总账失败: %w", err)
					}
				}
				return nil
			},
		},
	}
	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("finance-service 已启动 booking.status + order.status + payment.notify 消费者")
}
