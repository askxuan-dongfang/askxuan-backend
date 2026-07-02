package svc

import (
	"github.com/askxuan/diy-service/internal/config"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext diy 服务依赖容器
type ServiceContext struct {
	Config            config.Config
	DB                sqlx.SqlConn
	MqProducer        *mq.Producer
	Consumer          *mq.Consumer
	DiyDesignModel    model.DiyDesignModel
	DiyOrderModel     model.DiyOrderModel
	DiyOrderItemModel model.DiyOrderItemModel
	MaterialModel     model.MaterialModel
	MaterialSkuModel  model.MaterialSkuModel
	BlessingTaskModel model.BlessingTaskModel
	ExtraServiceModel model.ExtraServiceModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.DataSource)
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	consumer := mq.NewConsumer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:            c,
		DB:                db,
		MqProducer:        producer,
		Consumer:          consumer,
		DiyDesignModel:    model.NewDiyDesignModel(db),
		DiyOrderModel:     model.NewDiyOrderModel(db),
		DiyOrderItemModel: model.NewDiyOrderItemModel(db),
		MaterialModel:     model.NewMaterialModel(db),
		MaterialSkuModel:  model.NewMaterialSkuModel(db),
		BlessingTaskModel: model.NewBlessingTaskModel(db),
		ExtraServiceModel: model.NewExtraServiceModel(db),
	}
}
