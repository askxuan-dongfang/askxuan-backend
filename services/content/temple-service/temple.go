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
	startConsumer(ctx, svcCtx)

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
func startConsumer(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx == nil || svcCtx.Consumer == nil {
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

				// 查找加持任务记录
				task, err := svcCtx.BlessingTaskModel.FindByTaskNo(ctx, evt.TaskNo)
				if err != nil {
					logx.Errorf("查找加持任务失败 taskNo=%s: %v", evt.TaskNo, err)
					return nil
				}

				// 确定分配的法师
				masterCode := evt.MasterCode
				if masterCode == "" {
					// 派单未指定法师，使用默认法师
					masterCode = "M001"
				}

				// 分配法师：dispatched → assigned
				_, err = svcCtx.BlessingTaskModel.Assign(ctx, task.Id, masterCode)
				if err != nil {
					logx.Errorf("分配法师失败 taskNo=%s: %v", evt.TaskNo, err)
					return nil
				}

				// 发布 blessing.assign 事件通知法师服务
				err = svcCtx.MqProducer.PublishBlessingAssign(ctx, mq.BlessingAssign{
					TaskNo:      evt.TaskNo,
					TempleCode:  evt.TempleCode,
					MasterCode:  masterCode,
					ServiceCode: evt.ServiceCode,
				})
				if err != nil {
					logx.Errorf("发布 blessing.assign 失败 taskNo=%s: %v", evt.TaskNo, err)
				} else {
					logx.Infof("已分配法师 taskNo=%s masterCode=%s", evt.TaskNo, masterCode)
				}
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
				if evt.TargetType == "temple" {
					logx.Infof("寺院收到新评价通知: reviewId=%s templeId=%s action=%s", evt.ReviewId, evt.TargetId, evt.Action)
				}
				return nil
			},
		},
	}
	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("temple-service 已启动 blessing.dispatch 与 review.notify 消费者")
}
