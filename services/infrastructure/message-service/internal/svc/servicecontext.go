package svc

import (
	"github.com/askxuan/message-service/internal/config"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext message 服务依赖容器
type ServiceContext struct {
	Config            config.Config
	DB                sqlx.SqlConn
	Consumer          *mq.Consumer
	MessageModel      model.MessageModel
	TemplateModel     model.TemplateModel
	PushLogModel      model.PushLogModel
	AnnouncementModel model.AnnouncementModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:            c,
		DB:                db,
		Consumer:          consumer,
		MessageModel:      model.NewMessageModel(db),
		TemplateModel:     model.NewTemplateModel(db),
		PushLogModel:      model.NewPushLogModel(db),
		AnnouncementModel: model.NewAnnouncementModel(db),
	}
}
