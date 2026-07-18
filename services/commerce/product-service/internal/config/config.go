package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

// Config product 服务配置
type Config struct {
	rest.RestConf
	ProductRpc zrpc.RpcServerConf
	DataSource string // MySQL 数据源
	Redis      redis.RedisConf
}
