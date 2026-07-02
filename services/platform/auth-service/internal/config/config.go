package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
)

// MySQLConf MySQL 连接配置
type MySQLConf struct {
	DataSource string
}

// Config auth 服务配置结构
type Config struct {
	rest.RestConf
	MySQL MySQLConf
	Redis redis.RedisConf
	Auth  struct {
		AccessSecret  string
		AccessExpire  int64
		RefreshExpire int64
	}
	IM IMConf
}

// IMConf OpenIM 配置
type IMConf struct {
	APIURL      string
	AdminUserID string
	Secret      string
}
