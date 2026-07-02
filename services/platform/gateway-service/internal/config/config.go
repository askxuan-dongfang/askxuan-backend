package config

import "github.com/zeromicro/go-zero/rest"

// Upstream 下游服务路由规则
type Upstream struct {
	Prefix string // 路由前缀，如 /api/v1/temple
	Target string // 目标地址 host:port，如 localhost:8083
}

// Config 网关配置
type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
	}
	Upstreams   []Upstream
	NoAuthPaths []string
}
