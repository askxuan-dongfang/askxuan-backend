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
	startConsumer(ctx, svcCtx.Consumer)

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
func startConsumer(ctx context.Context, consumer *mq.Consumer) {
	if consumer == nil {
		return
	}
	bindings := []mq.Binding{
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
				if evt.Action == "completed" {
					model.InsertFinanceLog(model.FinanceLog{
						Type:        "income",
						Amount:      0,
						Description: fmt.Sprintf("预约收入:%s", evt.BookingId),
					})
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
				model.InsertFinanceLog(model.FinanceLog{
					Type:        "income",
					Amount:      evt.Amount,
					Description: fmt.Sprintf("支付%s:%s", evt.Action, evt.OrderNo),
				})
				return nil
			},
		},
	}
	consumer.Start(ctx, bindings)
	logx.Info("finance-service 已启动 booking.status + order.status + payment.notify 消费者")
}
