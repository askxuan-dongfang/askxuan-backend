package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/askxuan/media-service/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOProvider struct {
	client        *minio.Client
	bucket        string
	expires       int64
	publicBaseURL string
}

func NewMinIOProvider(c config.MinIOConf) (*MinIOProvider, error) {
	client, err := minio.New(c.Endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""), Secure: c.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	expires := c.PresignExpire
	if expires <= 0 {
		expires = 900
	}
	p := &MinIOProvider{client: client, bucket: c.Bucket, expires: expires, publicBaseURL: strings.TrimRight(c.PublicBaseURL, "/")}
	if err := p.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *MinIOProvider) Name() string { return "local_minio" }

func (p *MinIOProvider) PrepareUpload(ctx context.Context, objectName, contentType string) (string, map[string]string, int64, error) {
	presigned, err := p.client.PresignedPutObject(ctx, p.bucket, objectName, time.Duration(p.expires)*time.Second)
	if err != nil {
		return "", nil, 0, err
	}
	return presigned.String(), map[string]string{"Content-Type": contentType}, p.expires, nil
}

func (p *MinIOProvider) Stat(ctx context.Context, objectName string) (ObjectInfo, error) {
	info, err := p.client.StatObject(ctx, p.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Size: info.Size, ContentType: info.ContentType, ETag: info.ETag}, nil
}

func (p *MinIOProvider) PublicURL(objectName string) string {
	return p.publicBaseURL + "/" + strings.TrimLeft(objectName, "/")
}

func (p *MinIOProvider) ensureBucket(ctx context.Context) error {
	exists, err := p.client.BucketExists(ctx, p.bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := p.client.MakeBucket(ctx, p.bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, p.bucket)
	return p.client.SetBucketPolicy(ctx, p.bucket, policy)
}
