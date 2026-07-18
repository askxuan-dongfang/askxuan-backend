package svc

import (
	"github.com/askxuan/payment-service/internal/config"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/mq"
	"github.com/askxuan/payment-service/internal/rpcclient"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext payment 服务依赖容器
type ServiceContext struct {
	Config          config.Config
	DB              sqlx.SqlConn
	Redis           *redis.Redis
	MqProducer      *mq.Producer
	PaymentModel    model.PaymentModel
	PaymentLogModel model.PaymentLogModel
	RefundModel     model.RefundModel
	DiyOrderModel   model.DiyPaymentOrderModel
	ShopOrderClient rpcclient.ShopOrderClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.DataSource)
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:          c,
		DB:              db,
		Redis:           redis.MustNewRedis(c.Redis),
		MqProducer:      producer,
		PaymentModel:    model.NewPaymentModel(db),
		PaymentLogModel: model.NewPaymentLogModel(db),
		RefundModel:     model.NewRefundModel(db),
		DiyOrderModel:   model.NewDiyPaymentOrderModel(db),
		ShopOrderClient: rpcclient.NewShopOrderClient(zrpc.MustNewClient(c.OrderRpc)),
	}
}
