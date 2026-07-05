package svc

import (
	"github.com/askxuan/message-service/internal/config"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext message 服务依赖容器
type ServiceContext struct {
	Config            config.Config
	DB                sqlx.SqlConn
	Redis             *redis.Redis
	Consumer          *mq.Consumer
	MessageModel      model.MessageModel
	TemplateModel     model.TemplateModel
	PushLogModel      model.PushLogModel
	DeviceTokenModel  model.DeviceTokenModel
	AnnouncementModel model.AnnouncementModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	rds := redis.MustNewRedis(c.Redis)
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
		rds,
	)
	return &ServiceContext{
		Config:            c,
		DB:                db,
		Redis:             rds,
		Consumer:          consumer,
		MessageModel:      model.NewMessageModel(db),
		TemplateModel:     model.NewTemplateModel(db),
		PushLogModel:      model.NewPushLogModel(db),
		DeviceTokenModel:  model.NewDeviceTokenModel(db),
		AnnouncementModel: model.NewAnnouncementModel(db),
	}
}
