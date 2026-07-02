package config

import "github.com/zeromicro/go-zero/rest"

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
	RabbitMQ RabbitMQConf
	MySQL struct {
		DataSource string
	}
}
