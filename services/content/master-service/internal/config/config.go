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

// Config master 服务配置
type Config struct {
	rest.RestConf
	MySQL      MySQLConf
	RabbitMQ   RabbitMQConf
	AuthSecret string // JWT 签名密钥
	Redis      redis.RedisConf
	DiyRpc     zrpc.RpcClientConf // diy-service zrpc 客户端配置（查询 blessing_task）
}

// MySQLConf MySQL 数据源配置
type MySQLConf struct {
	DataSource string
}
