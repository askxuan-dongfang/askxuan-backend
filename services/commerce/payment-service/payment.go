package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/askxuan/payment-service/internal/config"
	"github.com/askxuan/payment-service/internal/handler"
	"github.com/askxuan/payment-service/internal/logic"
	"github.com/askxuan/payment-service/internal/mq"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"
	"github.com/askxuan/payment-service/rpc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/payment.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	if err := validatePaymentConfig(c); err != nil {
		panic(err)
	}

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)
	rpcServer := rpc.MustStartPaymentRpcServer(c, svcCtx)
	defer rpcServer.Stop()

	// 启动 RabbitMQ 消费者：监听 order.events 的 refund.request 消息
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startRefundRequestConsumer(ctx, c, svcCtx)
	mq.StartOutbox(ctx, svcCtx.DB, svcCtx.MqProducer)

	// 优雅退出
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	fmt.Printf("启动 payment-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

func validatePaymentConfig(c config.Config) error {
	env := strings.ToLower(strings.TrimSpace(c.AppEnv))
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if env == "prod" || env == "production" {
		return fmt.Errorf("production payment provider is not implemented; mock is forbidden")
	}
	if provider != "mock" {
		return fmt.Errorf("unsupported payment provider %q; only mock is implemented for local/test", provider)
	}
	return nil
}

// startRefundRequestConsumer 启动 refund.request 消费者。
//
// 消费 order.events exchange（fanout）中 Type=refund.request 的消息，
// 调用 RefundLogic 执行退款，完成后发送 refund.completed 事件到 payment.events。
// 队列 payment.refund.request 绑定到 order.events，按消息 Type 过滤。
func startRefundRequestConsumer(ctx context.Context, c config.Config, svcCtx *svc.ServiceContext) {
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
		svcCtx.Redis,
	)

	// RefundFunc 包装 RefundLogic.Refund，避免 mq ↔ logic ↔ svc 循环依赖
	refundFunc := func(ctx context.Context, paymentNo string, amount float64, reason string) (string, error) {
		resp, err := logic.NewRefundLogic(ctx, svcCtx).Refund(&types.RefundReq{
			PaymentNo: paymentNo,
			Amount:    amount,
			Reason:    reason,
		})
		if err != nil {
			return "", err
		}
		return resp.RefundNo, nil
	}

	bindings := []mq.Binding{
		{
			Exchange:         mq.ExchangeOrderEvents,
			Queue:            mq.QueuePaymentRefundRequest,
			MsgType:          mq.MessageTypeRefundRequest,
			EnableDeadLetter: true,
			Handler: mq.NewRefundRequestHandler(mq.RefundRequestDeps{
				RefundFunc: refundFunc,
				Redis:      svcCtx.Redis,
				MqProducer: svcCtx.MqProducer,
			}),
		},
	}
	consumer.Start(ctx, bindings)
	logx.Info("payment-service 已启动 refund.request 消费者")
}
