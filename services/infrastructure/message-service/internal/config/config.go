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

// MySQLConf MySQL 连接配置
type MySQLConf struct {
	DataSource string
}

// Config message 服务配置
type Config struct {
	rest.RestConf
	RabbitMQ   RabbitMQConf
	AuthSecret string // JWT 签名密钥
	MySQL      MySQLConf
	Redis      redis.RedisConf
}
