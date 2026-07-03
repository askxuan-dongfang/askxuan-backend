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

// MySQLConf MySQL 连接配置
type MySQLConf struct {
	DataSource string
}

// Config finance 服务配置
type Config struct {
	rest.RestConf
	MySQL      MySQLConf
	RabbitMQ   RabbitMQConf
	AuthSecret string // JWT 签名密钥
}
