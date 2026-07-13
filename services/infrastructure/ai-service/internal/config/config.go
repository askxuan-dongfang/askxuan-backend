package config

import (
	"github.com/zeromicro/go-zero/rest"
)

// MySQLConf MySQL 连接配置
type MySQLConf struct {
	DataSource string
}

// Config ai 服务配置
type Config struct {
	rest.RestConf
	MySQL MySQLConf
	AI    AIConf
}

type AIConf struct {
	Provider string
	BaseURL  string
	APIKey   string `json:",optional"`
	Model    string
}
