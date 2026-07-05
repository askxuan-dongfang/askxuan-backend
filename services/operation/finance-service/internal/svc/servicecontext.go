package svc

import (
	"github.com/askxuan/finance-service/internal/config"
	"github.com/askxuan/finance-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/redis"
)

// ServiceContext finance 服务依赖容器
type ServiceContext struct {
	Config     config.Config
	Redis      *redis.Redis
	MqProducer *mq.Producer
	Consumer   *mq.Consumer
}

func NewServiceContext(c config.Config) *ServiceContext {
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
		Config:     c,
		Redis:      rds,
		MqProducer: producer,
		Consumer:   consumer,
	}
}
