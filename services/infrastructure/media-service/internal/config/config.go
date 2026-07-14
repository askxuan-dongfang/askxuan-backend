package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	MySQL struct{ DataSource string }
	MinIO MinIOConf
	Media MediaConf
	Live  LiveConf
}

type MinIOConf struct {
	Endpoint        string
	PresignEndpoint string
	Region          string
	AccessKey       string
	SecretKey       string
	Bucket          string
	UseSSL          bool
	PresignExpire   int64
	PublicBaseURL   string
}

type MediaConf struct {
	Provider      string
	CallbackToken string
}

type LiveConf struct {
	Enabled  bool
	Provider string
}
