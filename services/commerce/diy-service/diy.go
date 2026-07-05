package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/askxuan/diy-service/internal/config"
	"github.com/askxuan/diy-service/internal/handler"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/mq"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/rpc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/diy.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	// 启动 gRPC server（供 master-service / temple-service 通过 zrpc 调用查询 blessing_task）
	rpcServer := rpc.MustStartDiyRpcServer(c, svcCtx)
	defer rpcServer.Stop()

	// 启动 RabbitMQ 消费者
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

	fmt.Printf("启动 diy-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startConsumer 启动 3 个 MQ 消费者：
//  1. blessing.complete — master-service 加持完成回传，更新 blessing_task + diy_order 状态
//  2. payment.notify    — payment-service 支付成功，diy_order pending_review → in_making
//  3. logistics.sync    — logistics-service 签收回传，diy_order shipped → completed
func startConsumer(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx.Consumer == nil {
		return
	}
	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangeBlessingEvents,
			Queue:    mq.QueueDiyBlessingComplete,
			Handler:  handleBlessingComplete(ctx, svcCtx),
		},
		{
			Exchange: mq.ExchangePaymentEvents,
			Queue:    mq.QueueDiyPaymentNotify,
			Handler:  handlePaymentNotify(ctx, svcCtx),
		},
		{
			Exchange: mq.ExchangeLogisticsEvents,
			Queue:    mq.QueueDiyLogisticsSync,
			Handler:  handleLogisticsSync(ctx, svcCtx),
		},
	}
	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("diy-service 已启动 3 个 MQ 消费者（blessing.complete / payment.notify / logistics.sync）")
}

// handleBlessingComplete 加持完成回传 → 更新 blessing_task 为 completed + diy_order 为 awaiting_shipment
func handleBlessingComplete(ctx context.Context, svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		evt, ok := mq.ParseBlessingComplete(body)
		if !ok {
			return nil
		}
		if evt.Status != "completed" {
			return nil
		}
		logx.Infof("收到加持完成回传: taskNo=%s diyOrderId=%s", evt.TaskNo, evt.DiyOrderId)

		// 1. 更新 blessing_task 状态为 completed
		task, err := svcCtx.BlessingTaskModel.FindByDiyOrderNo(ctx, evt.DiyOrderId)
		if err != nil {
			return nil // 幂等，不重投
		}
		if model.CanTransitBlessingTask(task.Status, model.BlessingTaskStatusCompleted) {
			_, _ = svcCtx.BlessingTaskModel.UpdateStatus(ctx, task.Id, model.BlessingTaskStatusCompleted)
		}

		// 2. 更新 diy_order 状态：awaiting_blessing → blessing_in_progress → blessing_completed → awaiting_shipment
		order, err := svcCtx.DiyOrderModel.FindByOrderNo(ctx, evt.DiyOrderId)
		if err != nil {
			return nil
		}
		if model.CanDiyTransit(order.Status, model.DiyStatusBlessingInProgress) {
			order, _ = svcCtx.DiyOrderModel.UpdateStatus(ctx, order.Id, model.DiyStatusBlessingInProgress)
		}
		if model.CanDiyTransit(order.Status, model.DiyStatusBlessingCompleted) {
			order, _ = svcCtx.DiyOrderModel.UpdateStatus(ctx, order.Id, model.DiyStatusBlessingCompleted)
		}
		if model.CanDiyTransit(order.Status, model.DiyStatusAwaitingShipment) {
			_, _ = svcCtx.DiyOrderModel.UpdateStatus(ctx, order.Id, model.DiyStatusAwaitingShipment)
		}
		return nil
	}
}

// handlePaymentNotify 支付成功 → diy_order pending_review → in_making
func handlePaymentNotify(ctx context.Context, svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		evt, ok := mq.ParsePaymentNotify(body)
		if !ok {
			return nil
		}
		if evt.Action != "success" || evt.OrderType != "diy_order" {
			return nil
		}
		logx.Infof("收到支付成功通知: orderNo=%s orderType=%s", evt.OrderNo, evt.OrderType)

		order, err := svcCtx.DiyOrderModel.FindByOrderNo(ctx, evt.OrderNo)
		if err != nil {
			return nil
		}
		if model.CanDiyTransit(order.Status, model.DiyStatusInMaking) {
			_, _ = svcCtx.DiyOrderModel.UpdateStatus(ctx, order.Id, model.DiyStatusInMaking)
		}
		return nil
	}
}

// handleLogisticsSync 物流签收 → diy_order shipped → completed
func handleLogisticsSync(ctx context.Context, svcCtx *svc.ServiceContext) func([]byte) error {
	return func(body []byte) error {
		evt, ok := mq.ParseLogisticsSync(body)
		if !ok {
			return nil
		}
		if evt.Status != "signed" || evt.OrderType != "diy_order" {
			return nil
		}
		logx.Infof("收到物流签收通知: orderId=%s orderType=%s", evt.OrderId, evt.OrderType)

		order, err := svcCtx.DiyOrderModel.FindByOrderNo(ctx, evt.OrderId)
		if err != nil {
			return nil
		}
		if model.CanDiyTransit(order.Status, model.DiyStatusCompleted) {
			_, _ = svcCtx.DiyOrderModel.UpdateStatus(ctx, order.Id, model.DiyStatusCompleted)
		}
		return nil
	}
}
