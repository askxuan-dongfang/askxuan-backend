package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/askxuan/logistics-service/internal/config"
	"github.com/askxuan/logistics-service/internal/handler"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/mq"
	"github.com/askxuan/logistics-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/logistics.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	// 启动 RabbitMQ 消费者：监听 order.events 发货事件
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

	fmt.Printf("启动 logistics-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startConsumer 启动订单发货事件消费者
func startConsumer(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx.Consumer == nil {
		return
	}
	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangeOrderEvents,
			Queue:    mq.QueueLogisticsOrderShipped,
			Handler: func(body []byte) error {
				evt, ok := mq.ParseOrderShipped(body)
				if !ok {
					return nil // 非 shipped 事件，跳过
				}
				// 根据订单号前缀区分业务类型（DIY 订单号以 "DIY" 开头）
				bizType := model.BizTypeOrder
				if strings.HasPrefix(evt.OrderId, "DIY") {
					bizType = model.BizTypeDiy
				}
				// 创建物流追踪记录
				_, err := svcCtx.LogisticsTrackModel.Insert(ctx, &model.LogisticsTrack{
					TrackingNo: evt.OrderId, // 用订单号作为临时物流单号
					BizType:    bizType,
					BizNo:      evt.OrderId,
					Status:     model.TrackStatusPending,
				})
				if err != nil {
					logx.Errorf("创建物流追踪记录失败(orderId=%s): %v", evt.OrderId, err)
					return err
				}
				logx.Infof("已为订单 %s 创建物流追踪记录 (bizType=%s)", evt.OrderId, bizType)
				return nil
			},
		},
	}
	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("logistics-service 已启动 order.events 消费者")
}
