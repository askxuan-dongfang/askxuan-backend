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
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/mq"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/rpc"

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
	rpcServer := rpc.MustStartMasterRpcServer(c, svcCtx)
	defer rpcServer.Stop()

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
// temple-service 分配法师后，master-service 校验任务可见性；实际状态由法师工作台推进。
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

				// 任务数据由 diy-service 持有。消费事件只确认法师侧可查询，不能代替法师操作。
				task, err := svcCtx.BlessingTaskModel.FindByTaskNo(ctx, evt.TaskNo)
				if err != nil {
					return fmt.Errorf("查找加持任务失败 taskNo=%s: %w", evt.TaskNo, err)
				}
				logx.Infof("加持任务已进入法师待处理列表: taskNo=%s id=%d status=%s", task.TaskNo, task.Id, task.Status)
				return nil
			},
		},
		{
			Exchange: mq.ExchangeFinanceEvents,
			Queue:    mq.QueueMasterBookingEarning,
			Handler: func(body []byte) error {
				evt, ok := mq.ParseSettlementAccrued(body)
				if !ok {
					return nil
				}
				if err := model.RecordBookingEarning(ctx, evt.SourceNo, evt.TargetId, evt.EarningDate, evt.ServiceName, evt.UserId, evt.Amount); err != nil {
					return fmt.Errorf("记录预约分成 %s 失败: %w", evt.SourceNo, err)
				}
				logx.Infof("平台预约分成已进入大师待结算收益 bookingId=%s masterCode=%s amount=%.2f", evt.SourceNo, evt.TargetId, evt.Amount)
				return nil
			},
		},
	}
	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("master-service 已启动 blessing.assign + finance settlement 消费者")
}
