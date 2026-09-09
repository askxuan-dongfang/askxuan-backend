package svc

import (
	"github.com/askxuan/marketing-service/internal/config"
	"github.com/askxuan/marketing-service/internal/model"
	"github.com/askxuan/marketing-service/internal/rewards"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext marketing 服务依赖容器
type ServiceContext struct {
	Config  config.Config
	Rewards rewards.Store
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.MySQL.DataSource)
	model.Configure(conn)
	db, err := conn.RawDB()
	if err != nil {
		panic(err)
	}
	return &ServiceContext{
		Config:  c,
		Rewards: rewards.Store{DB: db},
	}
}
