package config

import "github.com/zeromicro/go-zero/rest"

// Upstream 下游服务路由规则
// 同时支持静态 Target（向后兼容）与 ServiceName（etcd 动态发现），
// proxy 优先用动态发现实例，fallback 到静态 Target。
type Upstream struct {
	Prefix      string // 路由前缀，如 /api/v1/auth
	Target      string `json:",optional"` // 静态目标地址 host:port，如 localhost:8081
	ServiceName string `json:",optional"` // etcd 注册的 service key，如 auth.service
}

// Config 网关配置
type Config struct {
	rest.RestConf
	Auth struct {
		AccessSecret string
	}
	Etcd struct {
		Hosts []string
		Key   string
	}
	Upstreams   []Upstream
	NoAuthPaths []string
}
