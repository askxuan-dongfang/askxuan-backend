package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"github.com/askxuan/booking-service/internal/config"
	"github.com/askxuan/booking-service/internal/handler"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/booking-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/booking.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startPaymentSync(ctx, svcCtx)

	fmt.Printf("启动 booking-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

func startPaymentSync(ctx context.Context, svcCtx *svc.ServiceContext) {
	svcCtx.MqConsumer.Start(ctx, func(body []byte) error {
		var event mq.PaymentNotify
		if err := json.Unmarshal(body, &event); err != nil {
			return nil
		}
		if event.OrderType != "booking" || event.Action != "success" {
			return nil
		}
		updated, changed, err := svcCtx.BookingModel.UpdatePayment(ctx, event.OrderNo, event.PaymentNo, "mock", model.PaymentStatusSuccess, model.StatusPending)
		if err == nil && changed {
			recordRecoveredPayment(ctx, svcCtx, updated, "支付通知补偿确认")
		}
		return err
	})

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if count, err := svcCtx.BookingModel.ExpirePendingPayments(ctx); err != nil {
					logx.Errorf("取消支付超时预约失败: %v", err)
				} else if count > 0 {
					logx.Infof("已取消 %d 个支付超时预约并释放时段", count)
				}
				pending, err := svcCtx.BookingModel.FindPendingPayments(ctx, 100)
				if err != nil {
					logx.Errorf("查询待支付预约失败: %v", err)
					continue
				}
				for _, booking := range pending {
					payment, err := svcCtx.PaymentClient.GetOrderPayment(ctx, booking.Id)
					if err != nil || payment.Status != model.PaymentStatusSuccess {
						continue
					}
					updated, changed, err := svcCtx.BookingModel.UpdatePayment(ctx, booking.Id, payment.PaymentNo, payment.Channel, payment.Status, model.StatusPending)
					if err != nil {
						logx.Errorf("预约支付对账失败 booking=%s: %v", booking.Id, err)
					} else if changed {
						recordRecoveredPayment(ctx, svcCtx, updated, "支付定时对账补偿确认")
					}
				}
			}
		}
	}()
}

func recordRecoveredPayment(ctx context.Context, svcCtx *svc.ServiceContext, booking *model.Booking, remark string) {
	_ = svcCtx.StatusLogModel.Insert(ctx, &model.BookingStatusLog{BookingId: booking.Id, FromStatus: model.StatusPendingPayment, ToStatus: model.StatusPending, OperatorId: "booking-reconciler", OperatorType: model.OperatorTypeSystem, Remark: remark})
	if svcCtx.MqProducer != nil {
		_ = svcCtx.MqProducer.Publish(ctx, mq.BookingNotify{BookingId: booking.Id, UserId: booking.UserId, TempleId: booking.TempleId, Action: "created"})
	}
}
