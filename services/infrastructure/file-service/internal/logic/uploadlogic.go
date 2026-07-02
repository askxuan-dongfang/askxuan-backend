package logic

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/file-service/internal/svc"
	"github.com/askxuan/file-service/internal/types"

	"github.com/minio/minio-go/v7"
	"github.com/zeromicro/go-zero/core/logx"
)

// UploadLogic 文件上传逻辑（后端代传）
type UploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadLogic {
	return &UploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UploadFromReader 从 io.Reader 上传文件到 MinIO
// fileName: 原始文件名；contentType: MIME；size: 字节数；reader: 文件流
// 返回对象名与可访问 URL
func (l *UploadLogic) UploadFromReader(fileName, contentType string, size int64, reader io.Reader) (*types.UploadResp, error) {
	if l.svcCtx.MinIOClient == nil {
		return nil, common.NewBizError(7001, "MinIO 未就绪")
	}

	// 按业务目录 + 时间戳生成对象名，保留原始扩展名
	objectType := "temp"
	objectName := fmt.Sprintf("%s/%s_%s", objectType, time.Now().Format("20060102150405"), fileName)
	_ = filepath.Ext(fileName) // 保留扩展名已含在 fileName 中

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := l.svcCtx.MinIOClient.PutObject(l.ctx, l.svcCtx.Bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		l.Errorf("上传文件到 MinIO 失败: %v", err)
		return nil, common.ErrSystem
	}

	// 可访问 URL（bucket 已设公读策略，可直接通过 endpoint 访问）
	scheme := "http"
	if l.svcCtx.Config.MinIO.UseSSL {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s/%s/%s", scheme, l.svcCtx.Config.MinIO.Endpoint, l.svcCtx.Bucket, objectName)

	return &types.UploadResp{
		ObjectName:  objectName,
		Url:         url,
		Size:        size,
		ContentType: contentType,
	}, nil
}
