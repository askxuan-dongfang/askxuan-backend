package svc

import "github.com/askxuan/marketing-service/internal/config"

// ServiceContext marketing 服务依赖容器
type ServiceContext struct {
	Config config.Config
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}
