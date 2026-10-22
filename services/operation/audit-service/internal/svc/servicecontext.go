package svc

import (
	"github.com/askxuan/audit-service/internal/config"
	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/mq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext audit 服务依赖容器
type ServiceContext struct {
	Config     config.Config
	MqProducer *mq.Producer
	Operations model.OperationStore
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	model.Configure(conn)
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:     c,
		MqProducer: producer,
		Operations: model.NewOperationStore(conn),
	}
}
