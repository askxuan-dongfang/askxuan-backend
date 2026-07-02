package svc

import (
	"github.com/askxuan/review-service/internal/config"
	"github.com/askxuan/review-service/internal/mq"
)

// ServiceContext review 服务依赖容器
type ServiceContext struct {
	Config     config.Config
	MqProducer *mq.Producer
}

func NewServiceContext(c config.Config) *ServiceContext {
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:     c,
		MqProducer: producer,
	}
}
