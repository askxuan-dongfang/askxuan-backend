package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"os/signal"
	"syscall"

	"github.com/askxuan/order-service/internal/config"
	"github.com/askxuan/order-service/internal/handler"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/mq"
	"github.com/askxuan/order-service/internal/svc"
	orderrpc "github.com/askxuan/order-service/rpc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/order.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)
	rpcServer := orderrpc.MustStartOrderRpcServer(c, svcCtx)
	defer rpcServer.Stop()

	// 启动 RabbitMQ 消费者：监听 payment.notify 和 logistics.sync 事件
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startConsumer(ctx, svcCtx)

	// 启动 Outbox Publisher：轮询 outbox 表，将 pending 消息发送到 MQ
	startOutboxPublisher(ctx, svcCtx)

	// 优雅退出
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		cancel()
	}()

	fmt.Printf("启动 order-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

// startOutboxPublisher 启动 outbox publisher goroutine，轮询 outbox 表发送待投递消息
func startOutboxPublisher(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx.MqProducer == nil {
		logx.Info("MqProducer 未配置，跳过 outbox publisher 启动")
		return
	}
	publisher := mq.NewOutboxPublisher(svcCtx.DB, svcCtx.MqProducer)
	go publisher.Start(ctx)
	logx.Info("order-service 已启动 outbox publisher")
}

// startConsumer 启动支付通知和物流同步消费者
func startConsumer(ctx context.Context, svcCtx *svc.ServiceContext) {
	if svcCtx.Consumer == nil {
		return
	}
	bindings := []mq.Binding{
		{
			Exchange: mq.ExchangePaymentEvents,
			Queue:    mq.QueueOrderPaymentNotify,
			Handler: func(body []byte) error {
				var evt mq.PaymentNotify
				if err := json.Unmarshal(body, &evt); err != nil {
					logx.Errorf("解析 payment.notify 失败，丢弃: %v", err)
					return nil
				}
				logx.Infof("收到支付通知: paymentNo=%s orderType=%s orderNo=%s action=%s amount=%.2f",
					evt.PaymentNo, evt.OrderType, evt.OrderNo, evt.Action, evt.Amount)
				// 仅处理商城订单的支付成功事件
				if evt.OrderType != "shop_order" || evt.Action != "success" {
					return nil
				}
				o, err := svcCtx.ShopOrderModel.FindByOrderNo(ctx, evt.OrderNo)
				if err != nil {
					logx.Errorf("支付通知：查找订单失败 orderNo=%s: %v", evt.OrderNo, err)
					return nil // 不重投，避免重复处理
				}
				if !model.CanOrderTransit(o.Status, model.OrderStatusPaid) {
					logx.Infof("支付通知：订单 %s 状态 %s 无法流转到 paid，跳过", evt.OrderNo, o.Status)
					return nil
				}
				if _, err := svcCtx.ShopOrderModel.UpdateStatus(ctx, o.Id, model.OrderStatusPaid); err != nil {
					logx.Errorf("支付通知：更新订单状态失败 orderNo=%s: %v", evt.OrderNo, err)
					return err
				}
				logx.Infof("支付通知：订单 %s 已标记为已支付", evt.OrderNo)
				return nil
			},
		},
		{
			Exchange: mq.ExchangeLogisticsEvents,
			Queue:    mq.QueueOrderLogisticsSync,
			Handler: func(body []byte) error {
				var evt mq.LogisticsSync
				if err := json.Unmarshal(body, &evt); err != nil {
					logx.Errorf("解析 logistics.sync 失败，丢弃: %v", err)
					return nil
				}
				logx.Infof("收到物流同步: orderId=%s orderType=%s expressNo=%s status=%s",
					evt.OrderId, evt.OrderType, evt.ExpressNo, evt.Status)
				// 物流签收后自动完成订单（仅 shop_order）
				if evt.OrderType != "shop_order" || evt.Status != "signed" {
					return nil
				}
				o, err := svcCtx.ShopOrderModel.FindByOrderNo(ctx, evt.OrderId)
				if err != nil {
					logx.Errorf("物流同步：查找订单失败 orderNo=%s: %v", evt.OrderId, err)
					return nil
				}
				if !model.CanOrderTransit(o.Status, model.OrderStatusCompleted) {
					return nil
				}
				if _, err := svcCtx.ShopOrderModel.UpdateStatus(ctx, o.Id, model.OrderStatusCompleted); err != nil {
					logx.Errorf("物流同步：更新订单状态失败 orderNo=%s: %v", evt.OrderId, err)
					return err
				}
				logx.Infof("物流同步：订单 %s 已自动完成", evt.OrderId)
				return nil
			},
		},
		// 退款完成事件：payment-service 退款成功后通过 payment.events 发布 action=refunded，
		// order-service 消费后将 refunding 状态的退货单流转到 completed。
		// 独立队列 order.refund.completed，与 order.payment.notify 解耦避免相互影响。
		{
			Exchange: mq.ExchangePaymentEvents,
			Queue:    mq.QueueOrderRefundCompleted,
			Handler: mq.NewRefundCompletedHandler(mq.RefundCompletedDeps{
				FindRefunding: func(ctx context.Context, orderID int64) (*model.ReturnOrder, error) {
					var r model.ReturnOrder
					err := svcCtx.DB.QueryRowCtx(ctx, &r, `SELECT id,return_no,order_id,type,reason,status,refund_amount,create_time,update_time FROM return_order WHERE order_id=? AND status='refunding' ORDER BY id DESC LIMIT 1`, orderID)
					if errors.Is(err, sqlx.ErrNotFound) {
						return nil, nil
					}
					return &r, err
				},
				Finalize: func(ctx context.Context, o *model.ShopOrder, r *model.ReturnOrder) error {
					if o.RequestId != "" {
						if err := svcCtx.CatalogClient.ReleaseCart(ctx, o.RequestId); err != nil {
							return err
						}
					}
					return svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
						if _, err := tx.ExecCtx(ctx, `UPDATE return_order SET status='completed',update_time=NOW() WHERE id=? AND status='refunding'`, r.Id); err != nil {
							return err
						}
						_, err := tx.ExecCtx(ctx, `UPDATE shop_order SET status='cancelled',update_time=NOW() WHERE id=? AND status='in_return'`, o.Id)
						return err
					})
				},
				ShopOrderModel:   svcCtx.ShopOrderModel,
				ReturnOrderModel: svcCtx.ReturnOrderModel,
				Redis:            svcCtx.Redis,
			}),
		},
	}
	svcCtx.Consumer.Start(ctx, bindings)
	logx.Info("order-service 已启动 payment.notify + logistics.sync + refund.completed 消费者")
}
