package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/askxuan/master-service/internal/config"
	"github.com/askxuan/master-service/internal/handler"
	"github.com/askxuan/master-service/internal/mq"
	"github.com/askxuan/master-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/master.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	// 启动 RabbitMQ 消费者：监听 blessing.assign 分配事件
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

	fmt.Printf("启动 master-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startConsumer 启动 blessing.assign 消费者
// temple-service 分配法师后，master-service 接收并执行加持
func startConsumer(ctx context.Context, consumer *mq.Consumer) {
	if consumer == nil {
		return
	}
	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangeBlessingEvents,
			Queue:    mq.QueueMasterBlessingAssign,
			Handler: func(body []byte) error {
				evt, ok := mq.ParseBlessingAssign(body)
				if !ok {
					return nil // 非 blessing.assign 事件，跳过
				}
				logx.Infof("收到法师分配: taskNo=%s templeCode=%s masterCode=%s serviceCode=%s",
					evt.TaskNo, evt.TempleCode, evt.MasterCode, evt.ServiceCode)
				// TODO: 法师执行加持，完成后发布 blessing.complete 事件
				return nil
			},
		},
	}
	consumer.Start(ctx, bindings)
	logx.Info("master-service 已启动 blessing.assign 消费者")
}
