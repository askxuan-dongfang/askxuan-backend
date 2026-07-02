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
	startConsumer(ctx, svcCtx)

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
func startConsumer(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx == nil || svcCtx.Consumer == nil {
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

				// 查找加持任务
				task, err := svcCtx.BlessingTaskModel.FindByTaskNo(ctx, evt.TaskNo)
				if err != nil {
					logx.Errorf("查找加持任务失败 taskNo=%s: %v", evt.TaskNo, err)
					return nil
				}

				// 驱动状态机：assigned → accepted → in_progress
				if err := svcCtx.BlessingTaskModel.UpdateStatus(ctx, task.Id, "accepted"); err != nil {
					logx.Errorf("更新状态 accepted 失败: %v", err)
					return nil
				}
				if err := svcCtx.BlessingTaskModel.UpdateStatus(ctx, task.Id, "in_progress"); err != nil {
					logx.Errorf("更新状态 in_progress 失败: %v", err)
					return nil
				}

				// Mock 加持完成，生成证书 URL
				certURL := fmt.Sprintf("https://oss.askxuan.com/blessing/cert_%s.jpg", evt.TaskNo)
				certJSON := fmt.Sprintf(`["%s"]`, certURL)
				if err := svcCtx.BlessingTaskModel.UpdateComplete(ctx, task.Id, certJSON); err != nil {
					logx.Errorf("更新完成状态失败: %v", err)
					return nil
				}

				// 发布 blessing.complete 事件
				err = svcCtx.MqProducer.PublishBlessingComplete(ctx, mq.BlessingComplete{
					TaskNo:     evt.TaskNo,
					DiyOrderId: task.DiyOrderNo,
					TempleCode: evt.TempleCode,
					MasterCode: evt.MasterCode,
					Status:     "completed",
				})
				if err != nil {
					logx.Errorf("发布 blessing.complete 失败: %v", err)
				} else {
					logx.Infof("加持完成 taskNo=%s certUrl=%s", evt.TaskNo, certURL)
				}
				return nil
			},
		},
	}
	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("master-service 已启动 blessing.assign 消费者")
}
