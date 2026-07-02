package svc

import (
	"github.com/askxuan/finance-service/internal/config"
	"github.com/askxuan/finance-service/internal/mq"
)

// ServiceContext finance 服务依赖容器
type ServiceContext struct {
	Config     config.Config
	MqProducer *mq.Producer
	Consumer   *mq.Consumer
}

func NewServiceContext(c config.Config) *ServiceContext {
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:     c,
		MqProducer: producer,
		Consumer:   consumer,
	}
}
