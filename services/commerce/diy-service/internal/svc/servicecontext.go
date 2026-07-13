package svc

import (
	"github.com/askxuan/diy-service/internal/config"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext diy 服务依赖容器
type ServiceContext struct {
	Config                   config.Config
	DB                       sqlx.SqlConn
	Redis                    *redis.Redis
	MqProducer               *mq.Producer
	Consumer                 *mq.Consumer
	DiyDesignModel           model.DiyDesignModel
	DiyOrderModel            model.DiyOrderModel
	DiyOrderItemModel        model.DiyOrderItemModel
	CreatorEarningModel      model.CreatorEarningModel
	MaterialModel            model.MaterialModel
	MaterialSkuModel         model.MaterialSkuModel
	BlessingTaskModel        model.BlessingTaskModel
	ExtraServiceModel        model.ExtraServiceModel
	BlessingServiceListModel model.BlessingServiceListModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.DataSource)
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
		Config:                   c,
		DB:                       db,
		Redis:                    rds,
		MqProducer:               producer,
		Consumer:                 consumer,
		DiyDesignModel:           model.NewDiyDesignModel(db),
		DiyOrderModel:            model.NewDiyOrderModel(db),
		DiyOrderItemModel:        model.NewDiyOrderItemModel(db),
		CreatorEarningModel:      model.NewCreatorEarningModel(db),
		MaterialModel:            model.NewMaterialModel(db),
		MaterialSkuModel:         model.NewMaterialSkuModel(db),
		BlessingTaskModel:        model.NewBlessingTaskModel(db),
		ExtraServiceModel:        model.NewExtraServiceModel(db),
		BlessingServiceListModel: model.NewBlessingServiceListModel(),
	}
}
