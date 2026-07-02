package svc

import (
	"github.com/askxuan/master-service/internal/config"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext master 服务依赖容器
type ServiceContext struct {
	Config              config.Config
	MasterModel         model.MasterModel
	MasterAuditModel    model.MasterAuditModel
	MasterCredModel     model.MasterCredentialModel
	MasterScheduleModel model.MasterScheduleModel
	BlessingTaskModel   model.BlessingTaskModel
	MqProducer          *mq.Producer
	Consumer            *mq.Consumer
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)

	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:              c,
		MasterModel:         model.NewMasterModel(conn),
		MasterAuditModel:    model.NewMasterAuditModel(conn),
		MasterCredModel:     model.NewMasterCredentialModel(conn),
		MasterScheduleModel: model.NewMasterScheduleModel(conn),
		BlessingTaskModel:   model.NewBlessingTaskModel(conn),
		MqProducer:          producer,
		Consumer:            consumer,
	}
}
