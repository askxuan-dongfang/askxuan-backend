package svc

import (
	"github.com/askxuan/booking-service/internal/config"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/mq"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext booking 服务依赖容器
type ServiceContext struct {
	Config             config.Config
	DB                 sqlx.SqlConn
	MqProducer         *mq.Producer
	BookingModel       model.BookingModel
	StatusLogModel     model.BookingStatusLogModel
	ReviewModel        model.BookingReviewModel
	TempleReadonlyModel model.TempleReadonlyModel
	MasterReadonlyModel model.MasterReadonlyModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	// RabbitMQ 生产者采用懒连接，RabbitMQ 未启动时也不影响服务启动
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:              c,
		DB:                  db,
		MqProducer:          producer,
		BookingModel:        model.NewBookingModel(db),
		StatusLogModel:      model.NewBookingStatusLogModel(db),
		ReviewModel:         model.NewBookingReviewModel(db),
		TempleReadonlyModel: model.NewTempleReadonlyModel(db),
		MasterReadonlyModel: model.NewMasterReadonlyModel(db),
	}
}
