package svc

import (
	"context"
	"strconv"

	"github.com/askxuan/temple-service/internal/config"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/mq"
	"github.com/askxuan/temple-service/rpc/diy"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

// ============ 网关注入的请求头（gateway-service/internal/middleware/auth.go 约定） ============

const (
	HeaderUserID   = "X-User-Id"
	HeaderTempleID = "X-Temple-Id"
	HeaderRoles    = "X-User-Roles"
)

// ctxKey context key 类型，避免键冲突
type ctxKey string

// 预定义 context key
const (
	ctxKeyUserID   ctxKey = "userId"
	ctxKeyTempleID ctxKey = "templeId"
)

// WithUserID 将 userId 注入 context
func WithUserID(ctx context.Context, userId int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, userId)
}

// UserIDFromCtx 从 context 取出 userId（未登录返回 0）
func UserIDFromCtx(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyUserID).(int64); ok {
		return v
	}
	return 0
}

// WithTempleID 将 templeId 注入 context
func WithTempleID(ctx context.Context, templeId int64) context.Context {
	return context.WithValue(ctx, ctxKeyTempleID, templeId)
}

// TempleIDFromCtx 从 context 取出 templeId（非寺院管理员返回 0）
func TempleIDFromCtx(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyTempleID).(int64); ok {
		return v
	}
	return 0
}

// ParseHeaderToInt64 从请求头解析 int64 值
func ParseHeaderToInt64(val string) int64 {
	if val == "" {
		return 0
	}
	n, _ := strconv.ParseInt(val, 10, 64)
	return n
}

// ServiceContext temple 服务依赖容器
type ServiceContext struct {
	Config             config.Config
	DB                 sqlx.SqlConn
	Redis              *redis.Redis
	MqProducer         *mq.Producer
	Consumer           *mq.Consumer
	TempleModel        model.TempleModel
	BeliefModel        model.BeliefModel
	TempleImageModel   model.TempleImageModel
	TempleAdminModel   model.TempleAdminModel
	TempleAuditModel   model.TempleAuditModel
	TempleServiceModel model.TempleServiceModel
	BlessingTaskModel  model.BlessingTaskModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
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
		Config:             c,
		DB:                 db,
		Redis:              rds,
		MqProducer:         producer,
		Consumer:           consumer,
		TempleModel:        model.NewTempleModel(db),
		BeliefModel:        model.NewBeliefModel(db),
		TempleImageModel:   model.NewTempleImageModel(db),
		TempleAdminModel:   model.NewTempleAdminModel(db),
		TempleAuditModel:   model.NewTempleAuditModel(db),
		TempleServiceModel: model.NewTempleServiceModel(db),
		BlessingTaskModel:  model.NewBlessingTaskModel(diyClient),
	}
}
