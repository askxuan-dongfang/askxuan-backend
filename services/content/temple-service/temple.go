package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/askxuan/temple-service/internal/config"
	"github.com/askxuan/temple-service/internal/handler"
	"github.com/askxuan/temple-service/internal/mq"
	"github.com/askxuan/temple-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/temple.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	// 启动 RabbitMQ 消费者：监听 blessing.dispatch 派单事件
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

	fmt.Printf("启动 temple-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startConsumer 启动事件消费者
// 监听 blessing.dispatch 派单事件与 review.events 新评价通知
func startConsumer(ctx context.Context, consumer *mq.Consumer) {
	if consumer == nil {
		return
	}
	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangeBlessingEvents,
			Queue:    mq.QueueTempleBlessingDispatch,
			Handler: func(body []byte) error {
				evt, ok := mq.ParseBlessingDispatch(body)
				if !ok {
					return nil // 非 blessing.dispatch 事件，跳过
				}
				logx.Infof("收到加持派单: taskNo=%s diyOrderId=%s templeCode=%s serviceCode=%s",
					evt.TaskNo, evt.DiyOrderId, evt.TempleCode, evt.ServiceCode)
				// TODO: 创建加持任务记录，分配法师执行
				return nil
			},
		},
		{
			Exchange: mq.ExchangeReviewEvents,
			Queue:    mq.QueueTempleReviewNotify,
			Handler: func(body []byte) error {
				var evt mq.ReviewNotify
				if err := json.Unmarshal(body, &evt); err != nil {
					logx.Errorf("解析 review.notify 失败，丢弃: %v body=%s", err, string(body))
					return nil
				}
				logx.Infof("收到评价通知: reviewId=%s targetType=%s targetId=%s action=%s",
					evt.ReviewId, evt.TargetType, evt.TargetId, evt.Action)
				// TODO: 寺院管理台展示新评价，可按 targetType=temple 过滤本寺评价
				return nil
			},
		},
	}
	consumer.Start(ctx, bindings)
	logx.Info("temple-service 已启动 blessing.dispatch 与 review.notify 消费者")
}
