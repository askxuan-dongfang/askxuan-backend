package config

import (
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

// Config booking 服务配置
type Config struct {
	rest.RestConf
	RabbitMQ   RabbitMQConf
	AuthSecret string // JWT 签名密钥
	TempleRpc  zrpc.RpcClientConf
	MasterRpc  zrpc.RpcClientConf
	PaymentRpc zrpc.RpcClientConf
	MySQL      struct {
		DataSource string
	}
}
