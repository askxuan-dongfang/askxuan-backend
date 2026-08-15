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
	// GatewayBaseURL 网关地址（直约校验用，经网关读取大师公开详情；本地 localhost:8080，容器内自动改写）
	GatewayBaseURL string
	TempleRpc  zrpc.RpcClientConf
	MasterRpc  zrpc.RpcClientConf
	PaymentRpc zrpc.RpcClientConf
	IM         struct {
		APIURL      string
		AdminUserID string
		Secret      string
	}
	MySQL struct {
		DataSource string
	}
}
