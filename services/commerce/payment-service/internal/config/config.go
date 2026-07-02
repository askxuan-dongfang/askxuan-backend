package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

// RabbitMQConf RabbitMQ 连接配置
type RabbitMQConf struct {
	Host     string
	Port     int
	User     string
	Password string
	VHost    string
}

// WechatPayConf 微信支付配置
type WechatPayConf struct {
	AppId     string
	MchId     string
	ApiKey    string
	NotifyUrl string
}

// AlipayConf 支付宝配置
type AlipayConf struct {
	AppId      string
	PrivateKey string
	PublicKey  string
	NotifyUrl  string
}

// Config payment 服务配置
type Config struct {
	rest.RestConf
	DataSource string      // MySQL 数据源
	Redis      redis.RedisConf
	RabbitMQ   RabbitMQConf
	WechatPay  WechatPayConf
	Alipay     AlipayConf
}
