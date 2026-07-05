package svc

import (
	"github.com/askxuan/master-service/internal/config"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/mq"
	"github.com/askxuan/master-service/rpc/diy"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext master 服务依赖容器
type ServiceContext struct {
	Config              config.Config
	Redis               *redis.Redis
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

	// 通过 zrpc 调用 diy-service 查询 blessing_task（替代跨库直读 askxuan_diy.blessing_task）
	diyClient := diy.NewDiyService(zrpc.MustNewClient(c.DiyRpc))

	return &ServiceContext{
		Config:              c,
		Redis:               rds,
		MasterModel:         model.NewMasterModel(conn),
		MasterAuditModel:    model.NewMasterAuditModel(conn),
		MasterCredModel:     model.NewMasterCredentialModel(conn),
		MasterScheduleModel: model.NewMasterScheduleModel(conn),
		BlessingTaskModel:   model.NewBlessingTaskModel(diyClient),
		MqProducer:          producer,
		Consumer:            consumer,
	}
}
