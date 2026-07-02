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

// Config diy 服务配置
type Config struct {
	rest.RestConf
	DataSource string      // MySQL 数据源
	Redis      redis.RedisConf
	RabbitMQ   RabbitMQConf
}
