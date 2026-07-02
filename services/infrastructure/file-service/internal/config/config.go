package config

import "github.com/zeromicro/go-zero/rest"

// MinIOConf MinIO 配置
type MinIOConf struct {
	Endpoint      string
	AccessKey     string
	SecretKey     string
	Bucket        string
	UseSSL        bool
	PresignExpire int64 // 预签名 URL 有效期（秒）
}

// Config file 服务配置
type Config struct {
	rest.RestConf
	MinIO MinIOConf
}
