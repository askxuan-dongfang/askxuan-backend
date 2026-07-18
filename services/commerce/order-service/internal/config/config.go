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

// Config order 服务配置
type Config struct {
	rest.RestConf
	ProductRpc     zrpc.RpcClientConf
	OrderRpc       zrpc.RpcServerConf
	DataSource     string // MySQL 数据源
	Redis          redis.RedisConf
	RabbitMQ       RabbitMQConf
	AuthSecret     string // JWT 签名密钥，用于签名内部服务调用 token
	PaymentGateway string // payment-service 网关地址（含 host:port），为空则默认 http://localhost:8080
}
