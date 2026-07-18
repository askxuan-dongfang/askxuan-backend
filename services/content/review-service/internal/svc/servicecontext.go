package svc

import (
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/review-service/internal/config"
	"github.com/askxuan/review-service/internal/model"
	"github.com/askxuan/review-service/internal/mq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext review 服务依赖容器
type ServiceContext struct {
	Config     config.Config
	MqProducer *mq.Producer
	MqConsumer *mq.Consumer
	AuthConfig *middleware.AuthConfig
}

func NewServiceContext(c config.Config) *ServiceContext {
	model.Configure(sqlx.NewMysql(c.MySQL.DataSource))
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:     c,
		MqProducer: producer,
		MqConsumer: mq.NewConsumer(c.RabbitMQ.Host, c.RabbitMQ.Port, c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost),
		AuthConfig: &middleware.AuthConfig{Secret: c.AuthSecret},
	}
}
