package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/handler"
	"github.com/askxuan/ai-service/internal/mq"
	"github.com/askxuan/ai-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/ai.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	// 启动 RabbitMQ 消费者：监听 ai.divination 异步问事事件（自产自消费）
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

	fmt.Printf("启动 ai-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startConsumer 启动 ai.divination 消费者
// ai-service 自身发布的异步问事事件，由消费者调模型生成回复
func startConsumer(ctx context.Context, consumer *mq.Consumer) {
	if consumer == nil {
		return
	}
	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangeAIDivination,
			Queue:    mq.QueueAIDivination,
			Handler: func(body []byte) error {
				var evt mq.AIDivination
				if err := json.Unmarshal(body, &evt); err != nil {
					logx.Errorf("解析 ai.divination 失败，丢弃: %v", err)
					return nil
				}
				logx.Infof("收到异步问事: sessionId=%d userId=%s skillCode=%s content=%s",
					evt.SessionId, evt.UserId, evt.SkillCode, evt.Content)
				// TODO: 异步调用 AI 模型生成回复，写入会话消息
				return nil
			},
		},
	}
	consumer.Start(ctx, bindings)
	logx.Info("ai-service 已启动 ai.divination 消费者")
}
