package logic

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/file-service/internal/config"
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
		return nil, common.ErrOssService
	}
	if size <= 0 || size > 32<<20 {
		return nil, common.NewBizError(7002, "文件大小无效，最大 32MB")
	}
	// The public bucket may serve this object inline. Accept only safe raster images,
	// and derive the extension and MIME type from bytes rather than user headers.
	probe := make([]byte, 512)
	n, readErr := io.ReadFull(reader, probe)
	if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
		return nil, common.ErrSystem
	}
	contentType = http.DetectContentType(probe[:n])
	extensions := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}
	ext, ok := extensions[contentType]
	if !ok {
		return nil, common.NewBizError(7003, "仅支持 JPG、PNG、GIF、WebP 图片")
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, common.ErrSystem
	}
	objectName := "temp/" + hex.EncodeToString(id[:]) + ext
	reader = io.MultiReader(bytes.NewReader(probe[:n]), reader)
	_ = fileName // Original names never become public object keys.

	_, err := l.svcCtx.MinIOClient.PutObject(l.ctx, l.svcCtx.Bucket, objectName, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		l.Errorf("上传文件到 MinIO 失败: %v", err)
		return nil, common.ErrSystem
	}

	url := buildObjectURL(l.svcCtx.Config.MinIO, l.svcCtx.Bucket, objectName)

	return &types.UploadResp{
		ObjectName:  objectName,
		Url:         url,
		Size:        size,
		ContentType: contentType,
	}, nil
}

func buildObjectURL(config config.MinIOConf, bucket, objectName string) string {
	baseURL := strings.TrimRight(config.PublicBaseURL, "/")
	if baseURL == "" {
		scheme := "http"
		if config.UseSSL {
			scheme = "https"
		}
		baseURL = fmt.Sprintf("%s://%s", scheme, config.Endpoint)
	}
	return fmt.Sprintf("%s/%s/%s", baseURL, bucket, objectName)
}
