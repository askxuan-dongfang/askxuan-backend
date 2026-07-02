package svc

import (
	"context"
	"fmt"

	"github.com/askxuan/file-service/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/zeromicro/go-zero/core/logx"
)

// ServiceContext file 服务依赖容器
type ServiceContext struct {
	Config       config.Config
	MinIOClient  *minio.Client
	Bucket       string
	PresignExpire int64
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化 MinIO 客户端
	client, err := minio.New(c.MinIO.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.MinIO.AccessKey, c.MinIO.SecretKey, ""),
		Secure: c.MinIO.UseSSL,
	})
	if err != nil {
		// MinIO 客户端初始化失败不阻断启动，上传时再报错
		logx.Errorf("初始化 MinIO 客户端失败: %v", err)
	} else {
		// 自动创建 bucket（幂等，已存在则忽略）
		ensureBucket(client, c.MinIO.Bucket)
	}

	expire := c.MinIO.PresignExpire
	if expire <= 0 {
		expire = 900
	}

	return &ServiceContext{
		Config:        c,
		MinIOClient:   client,
		Bucket:        c.MinIO.Bucket,
		PresignExpire: expire,
	}
}

// ensureBucket 确保 bucket 存在，不存在则创建
func ensureBucket(client *minio.Client, bucket string) {
	if client == nil {
		return
	}
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		logx.Errorf("检查 bucket 存在性失败 bucket=%s: %v", bucket, err)
		return
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			logx.Errorf("创建 bucket 失败 bucket=%s: %v", bucket, err)
			return
		}
		// 设置 bucket 公读策略，便于前端通过对象 URL 直接访问（联调阶段）
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucket)
		_ = client.SetBucketPolicy(ctx, bucket, policy)
		logx.Infof("已创建 MinIO bucket: %s", bucket)
	}
}
