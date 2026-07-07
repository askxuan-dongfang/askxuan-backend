package svc

import (
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/review-service/internal/config"
	"github.com/askxuan/review-service/internal/mq"
)

// ServiceContext review 服务依赖容器
type ServiceContext struct {
	Config     config.Config
	MqProducer *mq.Producer
	AuthConfig *middleware.AuthConfig
}

func NewServiceContext(c config.Config) *ServiceContext {
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:     c,
		MqProducer: producer,
		AuthConfig: &middleware.AuthConfig{Secret: c.AuthSecret},
	}
}
