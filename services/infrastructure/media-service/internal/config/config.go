package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest"
)

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
	PresignUseSSL   bool `json:",optional"`
	Region          string
	AccessKey       string
	SecretKey       string
	Bucket          string
	UseSSL          bool
	PresignExpire   int64
	PublicBaseURL   string
}

func (c MinIOConf) Runtime() MinIOConf {
	if value := strings.TrimSpace(os.Getenv("MEDIA_MINIO_PRESIGN_ENDPOINT")); value != "" {
		c.PresignEndpoint = value
	}
	if value := strings.TrimSpace(os.Getenv("MEDIA_MINIO_PRESIGN_USE_SSL")); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			c.PresignUseSSL = parsed
		}
	}
	if value := strings.TrimSpace(os.Getenv("MEDIA_MINIO_PUBLIC_BASE_URL")); value != "" {
		c.PublicBaseURL = value
	}
	return c
}

type MediaConf struct {
	Provider      string
	CallbackToken string
}

type LiveConf struct {
	Enabled  bool
	Provider string
}
