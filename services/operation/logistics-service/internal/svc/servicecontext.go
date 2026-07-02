package svc

import (
	"github.com/askxuan/logistics-service/internal/config"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext logistics 服务依赖容器
type ServiceContext struct {
	Config               config.Config
	DB                   sqlx.SqlConn
	MqProducer           *mq.Producer
	Consumer             *mq.Consumer
	ExpressCompanyModel  model.ExpressCompanyModel
	FreightTemplateModel model.FreightTemplateModel
	LogisticsTrackModel  model.LogisticsTrackModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:               c,
		DB:                   db,
		MqProducer:           producer,
		Consumer:             consumer,
		ExpressCompanyModel:  model.NewExpressCompanyModel(db),
		FreightTemplateModel: model.NewFreightTemplateModel(db),
		LogisticsTrackModel:  model.NewLogisticsTrackModel(db),
	}
}
