package svc

import (
	"github.com/askxuan/booking-service/internal/config"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/booking-service/internal/rpcclient"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

// ServiceContext booking 服务依赖容器
type ServiceContext struct {
	Config         config.Config
	DB             sqlx.SqlConn
	MqProducer     *mq.Producer
	MqConsumer     *mq.Consumer
	BookingModel   model.BookingModel
	StatusLogModel model.BookingStatusLogModel
	ReviewModel    model.BookingReviewModel
	TempleClient   rpcclient.TempleClient
	MasterClient   rpcclient.MasterClient
	PaymentClient  rpcclient.PaymentClient
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	// RabbitMQ 生产者采用懒连接，RabbitMQ 未启动时也不影响服务启动
	producer := mq.NewProducer(
		c.RabbitMQ.Host, c.RabbitMQ.Port,
		c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost,
	)
	return &ServiceContext{
		Config:         c,
		DB:             db,
		MqProducer:     producer,
		MqConsumer:     mq.NewConsumer(c.RabbitMQ.Host, c.RabbitMQ.Port, c.RabbitMQ.User, c.RabbitMQ.Password, c.RabbitMQ.VHost),
		BookingModel:   model.NewBookingModel(db),
		StatusLogModel: model.NewBookingStatusLogModel(db),
		ReviewModel:    model.NewBookingReviewModel(db),
		TempleClient:   rpcclient.NewTempleClient(zrpc.MustNewClient(c.TempleRpc)),
		MasterClient:   rpcclient.NewMasterClient(zrpc.MustNewClient(c.MasterRpc)),
		PaymentClient:  rpcclient.NewPaymentClient(zrpc.MustNewClient(c.PaymentRpc)),
	}
}
