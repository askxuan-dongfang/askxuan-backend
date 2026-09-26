package svc

import (
	"github.com/askxuan/payment-service/internal/cashier"
	"github.com/askxuan/payment-service/internal/config"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/mq"
	"github.com/askxuan/payment-service/internal/paychannel"
	"github.com/askxuan/payment-service/internal/rpcclient"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext payment 服务依赖容器
type ServiceContext struct {
	Cashier         *cashier.Store
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
	producer := mq.NewProducer(db,
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	channels := map[string]paychannel.Gateway{}
	for name, cfg := range map[string]paychannel.Config{"wechat": c.Wallet.Wechat, "alipay": c.Wallet.Alipay} {
		gateway, err := paychannel.New(name, cfg)
		if err != nil {
			panic(err)
		}
		if gateway != nil {
			channels[name] = gateway
		}
	}
	return &ServiceContext{
		Cashier:         &cashier.Store{DB: db, Enabled: c.Wallet.Enabled, Mock: c.Provider == "mock", Channels: channels},
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
