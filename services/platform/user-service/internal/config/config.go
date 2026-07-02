package config

import "github.com/zeromicro/go-zero/rest"

// MySQLConf MySQL 数据源配置
type MySQLConf struct {
	DataSource string
}

// IMConf OpenIM 配置
type IMConf struct {
	APIURL      string
	AdminUserID string
	Secret      string
}

// Config user 服务配置
type Config struct {
	rest.RestConf
	MySQL MySQLConf
	Auth  struct {
		AccessSecret string
	}
	IM IMConf
}
