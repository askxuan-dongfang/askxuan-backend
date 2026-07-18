package svc

import (
	"github.com/askxuan/order-service/internal/config"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/mq"
	"github.com/askxuan/order-service/internal/rpcclient"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext order 服务依赖容器
type ServiceContext struct {
	Config                  config.Config
	DB                      sqlx.SqlConn
	Redis                   *redis.Redis
	MqProducer              *mq.Producer
	Consumer                *mq.Consumer
	ShopOrderModel          model.ShopOrderModel
	ShopOrderItemModel      model.ShopOrderItemModel
	ShopOrderLogisticsModel model.ShopOrderLogisticsModel
	ReturnOrderModel        model.ReturnOrderModel
	CatalogClient           rpcclient.CatalogClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.DataSource)
	rds := redis.MustNewRedis(c.Redis)
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
		rds,
	)
	return &ServiceContext{
		Config:                  c,
		DB:                      db,
		Redis:                   rds,
		MqProducer:              producer,
		Consumer:                consumer,
		ShopOrderModel:          model.NewShopOrderModel(db),
		ShopOrderItemModel:      model.NewShopOrderItemModel(db),
		ShopOrderLogisticsModel: model.NewShopOrderLogisticsModel(db),
		ReturnOrderModel:        model.NewReturnOrderModel(db),
		CatalogClient:           rpcclient.NewCatalogClient(zrpc.MustNewClient(c.ProductRpc)),
	}
}
