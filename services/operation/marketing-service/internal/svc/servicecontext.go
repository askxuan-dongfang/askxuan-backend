package svc

import (
	"github.com/askxuan/marketing-service/internal/config"
	"github.com/askxuan/marketing-service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext marketing 服务依赖容器
type ServiceContext struct {
	Config config.Config
}

func NewServiceContext(c config.Config) *ServiceContext {
	model.Configure(sqlx.NewMysql(c.MySQL.DataSource))
	return &ServiceContext{
		Config: c,
	}
}
