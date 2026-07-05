package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

// RabbitMQConf RabbitMQ 连接配置
type RabbitMQConf struct {
	Host     string
	Port     int
	User     string
	Password string
	VHost    string
}

// Config diy 服务配置
type Config struct {
	rest.RestConf
	DataSource string      // MySQL 数据源
	Redis      redis.RedisConf
	RabbitMQ   RabbitMQConf
	DiyRpc     zrpc.RpcServerConf // gRPC server（供 master/temple 服务通过 zrpc 调用查询 blessing_task）
}
