package svc

import (
	"github.com/askxuan/audit-service/internal/config"
	"github.com/askxuan/audit-service/internal/mq"
)

// ServiceContext audit 服务依赖容器
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
